/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package ddl

import (
	"context"
)

import (
	"github.com/arana-db/arana/pkg/proto"
	"github.com/arana-db/arana/pkg/proto/rule"
	"github.com/arana-db/arana/pkg/runtime/ast"
	"github.com/arana-db/arana/pkg/runtime/optimize"
	"github.com/arana-db/arana/pkg/runtime/plan/ddl"
	"github.com/arana-db/arana/pkg/runtime/plan/dml"
	"github.com/arana-db/arana/pkg/util/log"
)

func init() {
	optimize.Register(ast.SQLTypeCreateTable, optimizeCreateTable)
}

func optimizeCreateTable(ctx context.Context, o *optimize.Optimizer) (proto.Plan, error) {
	stmt := o.Stmt.(*ast.CreateTableStmt)

	var (
		shards   rule.DatabaseTables
		fullScan bool
	)
	vt, ok := o.Rule.VTable(stmt.Table.Suffix())
	fullScan = ok

	log.Debugf("compute shards: result=%s, isFullScan=%v", shards, fullScan)

	toSingle := func(db, tbl string) (proto.Plan, error) {
		ret := &ddl.CreateTablePlan{
			Stmt:     stmt,
			Database: db,
			Tables:   []string{tbl},
		}
		ret.BindArgs(o.Args)

		return ret, nil
	}

	// Go through first table if not full scan.
	if !fullScan {
		return toSingle("", stmt.Table.Suffix())
	}

	// expand all shards if all shards matched
	shards = vt.Topology().Enumerate()

	plans := make([]proto.Plan, 0, len(shards))
	for k, v := range shards {
		next := &ddl.CreateTablePlan{
			Database: k,
			Tables:   v,
			Stmt:     stmt,
		}
		next.BindArgs(o.Args)
		plans = append(plans, next)
	}

	tmpPlan := &dml.CompositePlan{
		Plans: plans,
	}

	return tmpPlan, nil
}
