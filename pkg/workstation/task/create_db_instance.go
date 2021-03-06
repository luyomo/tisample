// Copyright 2020 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package task

import (
	"context"
	"encoding/json"
	"fmt"
	//	"github.com/luyomo/tisample/pkg/workstation/ctxt"
	"github.com/luyomo/tisample/pkg/workstation/executor"
	//	"strconv"
	"strings"
	"time"
)

type DBInstance struct {
	DBInstanceIdentifier string `json:"DBInstanceIdentifier"`
	DBInstanceStatus     string `json:"DBInstanceStatus"`
}

type NewDBInstance struct {
	DBInstance DBInstance `json:"DBInstance"`
}

type DBInstances struct {
	DBInstances []DBInstance `json:"DBInstances"`
}

type CreateDBInstance struct {
	user string
	host string
}

// Execute implements the Task interface
func (c *CreateDBInstance) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	// Get the available zones
	command := fmt.Sprintf("aws rds describe-db-instances --db-instance-identifier '%s'", "tisampletest")
	stdout, stderr, err := local.Execute(ctx, command, false)
	if err != nil {
		if strings.Contains(string(stderr), fmt.Sprintf("DBInstance %s not found", "tisampletest")) {
			fmt.Printf("The DB Instance has not created.\n\n\n")
		} else {
			fmt.Printf("The error err here is <%#v> \n\n", err)
			fmt.Printf("----------\n\n")
			fmt.Printf("The error stderr here is <%s> \n\n", string(stderr))
			return nil
		}
	} else {
		fmt.Printf("The DB Instance has been created\n\n\n")
		return nil
	}

	fmt.Printf("The DB instance  <%s> \n\n\n", string(stdout))

	command = fmt.Sprintf("aws rds create-db-instance --db-instance-identifier %s --db-cluster-identifier %s --db-parameter-group-name db-params-%s --engine aurora-mysql --engine-version 5.7.12 --db-instance-class db.r5.large", "tisampletest", "tisampletest", "tisampletest")
	fmt.Printf("The comamnd is <%s> \n\n\n", command)
	stdout, stderr, err = local.Execute(ctx, command, false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}

	fmt.Printf("The db instance is <%#v>\n\n\n", string(stdout))

	var newDBInstance NewDBInstance
	if err = json.Unmarshal(stdout, &newDBInstance); err != nil {
		fmt.Printf("*** *** The error here is %#v \n\n", err)
		return nil
	}
	fmt.Printf("The db instance is <%#v>\n\n\n", newDBInstance)

	for i := 1; i <= 50; i++ {
		command := fmt.Sprintf("aws rds describe-db-instances --db-instance-identifier '%s'", "tisampletest")
		stdout, stderr, err := local.Execute(ctx, command, false)
		if err != nil {
			fmt.Printf("The error err here is <%#v> \n\n", err)
			fmt.Printf("----------\n\n")
			fmt.Printf("The error stderr here is <%s> \n\n", string(stderr))
			return nil
		}
		//fmt.Printf("The db cluster is <%#v>\n\n\n", string(stdout))
		var dbInstances DBInstances
		if err = json.Unmarshal(stdout, &dbInstances); err != nil {
			fmt.Printf("*** *** The error here is %#v \n\n", err)
			return nil
		}
		fmt.Printf("The db cluster is <%#v>\n\n\n", dbInstances)
		if dbInstances.DBInstances[0].DBInstanceStatus == "available" {
			break
		}
		time.Sleep(20 * time.Second)
	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateDBInstance) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateDBInstance) String() string {
	return fmt.Sprintf("Echo: Generating the DB instance %s ", "tisampletest")
}
