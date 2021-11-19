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
	"github.com/luyomo/tisample/pkg/aws/ctxt"
	"github.com/luyomo/tisample/pkg/aws/executor"
	"github.com/luyomo/tisample/pkg/aws/spec"
	"go.uber.org/zap"
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

func (s Subnet) String() string {
	return fmt.Sprintf("AvailabilityZone:%s, CidrBlock:%s, State:%s, SubnetId: %s, VpcId: %s", s.AvailabilityZone, s.CidrBlock, s.State, s.SubnetId, s.VpcId)
}

func (s Subnets) String() string {
	var res []string
	for _, subnet := range s.Subnets {
		res = append(res, subnet.String())
	}
	return fmt.Sprintf("Subnets: [%s]", strings.Join(res, ","))
}

func (r SubnetResult) String() string {
	return fmt.Sprintf("SubnetResult: [%s]", r.String())
}

// Mkdir is used to create directory on the target host
type CreateNetwork struct {
	user           string
	host           string
	awsTopoConfigs *spec.AwsTopoConfigs
	clusterName    string
	clusterType    string
}

// Execute implements the Task interface
func (c *CreateNetwork) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	if err != nil {
		return err
	}

	zones, err := getAvailableZones(local, ctx)
	if err != nil {
		return nil
	}

	zap.L().Debug("Public Route Table ID", zap.String("publicRouteTableId", clusterInfo.publicRouteTableId))
	zap.L().Debug("Private Route Table ID", zap.String("privateRouteTableId", clusterInfo.privateRouteTableId))

	c.createPrivateSubnets(local, ctx, zones)

	c.createPublicSubnets(local, ctx, zones)

	zap.L().Debug("Public Route Table ID", zap.String("privateSubnets", strings.Join(clusterInfo.privateSubnets, ",")))
	zap.L().Debug("Private Route Table ID", zap.String("privateSubnets", strings.Join(clusterInfo.privateSubnets, ",")))

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

	return ipSegs[0] + "." + ipSegs[1] + "." + strconv.Itoa(idx) + ".0/24"
}

func associateSubnet2RouteTable(subnet string, routeTableId string, executor ctxt.Executor, ctx context.Context) {
	command := fmt.Sprintf("aws ec2 associate-route-table --route-table-id %s --subnet-id %s ", routeTableId, subnet)
	zap.L().Debug("Associating route table", zap.String("command", command))
	if _, _, err := executor.Execute(ctx, command, false); err != nil {
		return
	}
}

func getAvailableZones(executor ctxt.Executor, ctx context.Context) (AvailabilityZones, error) {

	// Get the available zones
	stdout, _, err := executor.Execute(ctx, "aws ec2 describe-availability-zones", false)
	if err != nil {
		return AvailabilityZones{}, err
	}
	//fmt.Printf("The stdout from the local is <%s> \n\n", string(stdout))
	var zones AvailabilityZones
	if err = json.Unmarshal(stdout, &zones); err != nil {
		zap.L().Error("Failed to parse json", zap.Error(err))
		return AvailabilityZones{}, err
	}
	return zones, nil
}

func (c *CreateNetwork) createPrivateSubnets(executor ctxt.Executor, ctx context.Context, zones AvailabilityZones) error {
	// Get the subnets
	command := fmt.Sprintf("aws ec2 describe-subnets --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Scope\" \"Name=tag-value,Values=private\"", c.clusterName, c.clusterType)
	zap.L().Debug("Command", zap.String("describe-subnets", command))
	stdout, _, err := executor.Execute(ctx, command, false)
	if err != nil {
		return nil
	}

	var subnets Subnets
	if err = json.Unmarshal(stdout, &subnets); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("subnets", string(stdout)))
		return nil
	}
	for idx, zone := range zones.Zones {
		subnetExists := false
		for idxNet, subnet := range subnets.Subnets {
			if zone.ZoneName == subnet.AvailabilityZone {
				zap.L().Info("avaiabilityZone", zap.Int("idxNet", idxNet), zap.String("availability zone", subnet.AvailabilityZone))
				clusterInfo.privateSubnets = append(clusterInfo.privateSubnets, subnet.SubnetId)
				associateSubnet2RouteTable(subnet.SubnetId, clusterInfo.privateRouteTableId, executor, ctx)
				subnetExists = true
			}
		}
		if subnetExists == true {
			continue
		}

		command := fmt.Sprintf("aws ec2 create-subnet --cidr-block %s --vpc-id %s --availability-zone=%s --tag-specifications \"ResourceType=subnet,Tags=[{Key=Name,Value=%s},{Key=Type,Value=%s},{Key=Scope,Value=private}]\"", getNextCidr(clusterInfo.vpcInfo.CidrBlock, idx+1), clusterInfo.vpcInfo.VpcId, zone.ZoneName, c.clusterName, c.clusterType)
		zap.L().Debug("Command", zap.String("create-subnets", command))

		stdout, _, err := executor.Execute(ctx, command, false)
		if err != nil {
			return nil
		}
		var newSubnet SubnetResult
		if err = json.Unmarshal(stdout, &newSubnet); err != nil {
			//			fmt.Printf("*** *** The error here is %#v \n\n\n", err)
			zap.L().Debug("Json unmarshal", zap.String("subnets", string(stdout)))
			return nil
		}
		zap.L().Debug("Generated the subnet info", zap.String("State", newSubnet.Subnet.State), zap.String("Cidr Block", newSubnet.Subnet.CidrBlock))
		associateSubnet2RouteTable(newSubnet.Subnet.SubnetId, clusterInfo.privateRouteTableId, executor, ctx)
		clusterInfo.privateSubnets = append(clusterInfo.privateSubnets, newSubnet.Subnet.SubnetId)
	}

	return nil
}

func (c *CreateNetwork) createPublicSubnets(executor ctxt.Executor, ctx context.Context, zones AvailabilityZones) error {
	// Get the subnets
	command := fmt.Sprintf("aws ec2 describe-subnets --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Scope\" \"Name=tag-value,Values=public\"", c.clusterName, c.clusterType)
	zap.L().Debug("Command", zap.String("describe-subnets", command))
	stdout, _, err := executor.Execute(ctx, command, false)
	if err != nil {
		return nil
	}
	var subnets Subnets
	if err = json.Unmarshal(stdout, &subnets); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("subnets", string(stdout)))
		return nil
	}

	if len(subnets.Subnets) > 0 {
		clusterInfo.publicSubnet = subnets.Subnets[0].SubnetId
		zap.L().Debug("Public subnets ", zap.String("subnet", clusterInfo.publicSubnet))
		return nil
	}

	command = fmt.Sprintf("aws ec2 create-subnet --cidr-block %s --vpc-id %s --availability-zone=%s --tag-specifications \"ResourceType=subnet,Tags=[{Key=Name,Value=%s},{Key=Type,Value=%s},{Key=Scope,Value=public}]\"", getNextCidr(clusterInfo.vpcInfo.CidrBlock, 10+1), clusterInfo.vpcInfo.VpcId, zones.Zones[0].ZoneName, c.clusterName, c.clusterType)
	zap.L().Debug("Command", zap.String("create-subnet", command))
	stdout, _, err = executor.Execute(ctx, command, false)
	if err != nil {
		return nil
	}
	var newSubnet SubnetResult
	if err = json.Unmarshal(stdout, &newSubnet); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("subnet", string(stdout)))
		return nil
	}
	zap.L().Debug("Generated the subnet info", zap.String("State", newSubnet.Subnet.State), zap.String("Cidr Block", newSubnet.Subnet.CidrBlock))
	associateSubnet2RouteTable(newSubnet.Subnet.SubnetId, clusterInfo.publicRouteTableId, executor, ctx)
	clusterInfo.publicSubnet = newSubnet.Subnet.SubnetId

	return nil
}
