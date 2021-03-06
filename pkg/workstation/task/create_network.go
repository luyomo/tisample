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
	"github.com/luyomo/tisample/pkg/workstation/ctxt"
	"github.com/luyomo/tisample/pkg/workstation/executor"
	"strconv"
	"strings"
	//"time"
)

type RegionZone struct {
	RegionName string `json:"RegionName"`
	ZoneName   string `json:"ZoneName"`
}

type AvailabilityZones struct {
	Zones []RegionZone `json:"AvailabilityZones"`
}

type Subnet struct {
	AvailabilityZone string `json:"AvailabilityZone"`
	CidrBlock        string `json:"CidrBlock"`
	State            string `json:"State"`
	SubnetId         string `json:"SubnetId"`
	VpcId            string `json:"VpcId"`
}

type Subnets struct {
	Subnets []Subnet `json:"Subnets"`
}

type SubnetResult struct {
	Subnet Subnet `json:"Subnet"`
}

// Mkdir is used to create directory on the target host
type CreateNetwork struct {
	user string
	host string
}

// Execute implements the Task interface
func (c *CreateNetwork) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	fmt.Printf("The type of local is <%T> \n\n\n", local)
	// Get the available zones
	stdout, stderr, err := local.Execute(ctx, "aws ec2 describe-availability-zones", false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}
	//fmt.Printf("The stdout from the local is <%s> \n\n", string(stdout))
	var zones AvailabilityZones
	if err = json.Unmarshal(stdout, &zones); err != nil {
		fmt.Printf("*** *** The error here is %#v \n\n", err)
		return nil
	}
	//	fmt.Printf("*** *** *** The parsed data is \n %#v \n\n", zones.Zones)
	//	fmt.Printf("The length of the zones is <%d> \n\n", len(zones.Zones))
	//fmt.Println("--------------------------- \n")

	// Get the subnets
	stdout, stderr, err = local.Execute(ctx, "aws ec2 describe-subnets --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=tisamplews\"", false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}
	//fmt.Printf("The stdout from the local is <%s> \n\n\n", string(stdout))
	var subnets Subnets
	if err = json.Unmarshal(stdout, &subnets); err != nil {
		fmt.Printf("*** *** The error here is %#v \n\n", err)
		return nil
	}
	if len(subnets.Subnets) > 0 {
		fmt.Printf("*** *** *** Got the subnets <%#v> \n\n\n", subnets)
		clusterInfo.subnet = subnets.Subnets[0].SubnetId
		return nil
	}
	command := fmt.Sprintf("aws ec2 create-subnet --cidr-block %s --vpc-id %s --availability-zone=%s --tag-specifications \"ResourceType=subnet,Tags=[{Key=Name,Value=tisamplews}]\"", getNextCidr(clusterInfo.vpcInfo.CidrBlock, 1), clusterInfo.vpcInfo.VpcId, zones.Zones[0].ZoneName)
	fmt.Printf("The comamnd is <%s> \n\n\n", command)
	sub_stdout, sub_stderr, sub_err := local.Execute(ctx, command, false)
	if sub_err != nil {
		fmt.Printf("The error here is <%#v> \n\n", sub_err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(sub_stderr))
		return nil
	}
	var newSubnet SubnetResult
	if err = json.Unmarshal(sub_stdout, &newSubnet); err != nil {
		fmt.Printf("*** *** The error here is %#v \n\n\n", err)
		return nil
	}
	associateSubnet2RouteTable(newSubnet.Subnet.SubnetId, clusterInfo.routeTableId, local, ctx)
	clusterInfo.subnet = newSubnet.Subnet.SubnetId
	//fmt.Printf("The stdout from the subnett preparation: %s \n\n\n", sub_stdout)
	//fmt.Printf("The stdout from the subnett preparation: %s and %s \n\n\n", newSubnet.Subnet.State, newSubnet.Subnet.CidrBlock)
	//	associateSubnet2RouteTable(newSubnet.Subnet.SubnetId, clusterInfo.routeTableId, local, ctx)
	//	clusterInfo.subnets = append(clusterInfo.subnets, newSubnet.Subnet.SubnetId)

	return nil
}

// Rollback implements the Task interface
func (c *CreateNetwork) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateNetwork) String() string {
	return fmt.Sprintf("Echo: host=%s ", c.host)
}

func getNextCidr(cidr string, idx int) string {
	ip := strings.Split(cidr, "/")[0]
	ipSegs := strings.Split(ip, ".")
	//	maskLen := strings.Split(cidr, "/")[1]
	return ipSegs[0] + "." + ipSegs[1] + "." + strconv.Itoa(idx) + ".0/24"
}

func associateSubnet2RouteTable(subnet string, routeTableId string, executor ctxt.Executor, ctx context.Context) {
	command := fmt.Sprintf("aws ec2 associate-route-table --route-table-id %s --subnet-id %s ", routeTableId, subnet)
	fmt.Printf("The comamnd is <%s> \n\n\n", command)
	stdout, stderr, err := executor.Execute(ctx, command, false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
	}
	fmt.Printf("The stdout is <%s>\n\n\n", stdout)
	fmt.Printf("The stderr is <%s>\n\n\n", stderr)
}
