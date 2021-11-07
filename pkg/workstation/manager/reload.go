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

package manager

import (
	"context"
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	"github.com/luyomo/tisample/pkg/workstation/clusterutil"
	"github.com/luyomo/tisample/pkg/workstation/ctxt"
	"github.com/luyomo/tisample/pkg/workstation/executor"
	operator "github.com/luyomo/tisample/pkg/workstation/operation"
	"github.com/luyomo/tisample/pkg/workstation/spec"
	"github.com/luyomo/tisample/pkg/logger/log"
	"github.com/luyomo/tisample/pkg/set"
	"github.com/luyomo/tisample/pkg/tui"
	perrs "github.com/pingcap/errors"
)

// Reload the cluster.
func (m *Manager) Reload(name string, gOpt operator.Options, skipRestart, skipConfirm bool) error {
	if err := clusterutil.ValidateClusterNameOrError(name); err != nil {
		return err
	}

	sshTimeout := gOpt.SSHTimeout
	exeTimeout := gOpt.OptTimeout

	metadata, err := m.meta(name)
	if err != nil {
		return err
	}

	var sshProxyProps *tui.SSHConnectionProps = &tui.SSHConnectionProps{}
	if gOpt.SSHType != executor.SSHTypeNone && len(gOpt.SSHProxyHost) != 0 {
		var err error
		if sshProxyProps, err = tui.ReadIdentityFileOrPassword(gOpt.SSHProxyIdentity, gOpt.SSHProxyUsePassword); err != nil {
			return err
		}
	}

	if !skipConfirm {
		if err := tui.PromptForConfirmOrAbortError(
			fmt.Sprintf("Will reload the cluster %s with restart policy is %s, nodes: %s, roles: %s.\nDo you want to continue? [y/N]:",
				color.HiYellowString(name),
				color.HiRedString(fmt.Sprintf("%v", !skipRestart)),
				color.HiRedString(strings.Join(gOpt.Nodes, ",")),
				color.HiRedString(strings.Join(gOpt.Roles, ",")),
			),
		); err != nil {
			return err
		}
	}

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()

	uniqueHosts := make(map[string]hostInfo) // host -> ssh-port, os, arch
	noAgentHosts := set.NewStringSet()
	topo.IterInstance(func(inst spec.Instance) {
		// add the instance to ignore list if it marks itself as ignore_exporter
		if inst.IgnoreMonitorAgent() {
			noAgentHosts.Insert(inst.GetHost())
		}

		if _, found := uniqueHosts[inst.GetHost()]; !found {
			uniqueHosts[inst.GetHost()] = hostInfo{
				ssh:  inst.GetSSHPort(),
				os:   inst.OS(),
				arch: inst.Arch(),
			}
		}
	})

	refreshConfigTasks, hasImported := buildRegenConfigTasks(m, name, topo, base, nil, gOpt.IgnoreConfigCheck)
	monitorConfigTasks := buildRefreshMonitoredConfigTasks(
		m.specManager,
		name,
		uniqueHosts,
		noAgentHosts,
		*topo.BaseTopo().GlobalOptions,
		topo.GetMonitoredOptions(),
		sshTimeout,
		exeTimeout,
		gOpt,
		sshProxyProps,
	)

	// handle dir scheme changes
	if hasImported {
		if err := spec.HandleImportPathMigration(name); err != nil {
			return err
		}
	}

	b, err := m.sshTaskBuilder(name, topo, base.User, gOpt)
	if err != nil {
		return err
	}
	if topo.Type() == spec.TopoTypeTiDB && !skipRestart {
		b.UpdateTopology(
			name,
			m.specManager.Path(name),
			metadata.(*spec.ClusterMeta),
			nil, /* deleteNodeIds */
		)
	}
	b.ParallelStep("+ Refresh instance configs", gOpt.Force, refreshConfigTasks...)

	if len(monitorConfigTasks) > 0 {
		b.ParallelStep("+ Refresh monitor configs", gOpt.Force, monitorConfigTasks...)
	}

	if !skipRestart {
		tlsCfg, err := topo.TLSConfig(m.specManager.Path(name, spec.TLSCertKeyDir))
		if err != nil {
			return err
		}
		b.Func("UpgradeCluster", func(ctx context.Context) error {
			return operator.Upgrade(ctx, topo, gOpt, tlsCfg)
		})
	}

	t := b.Build()

	if err := t.Execute(ctxt.New(context.Background(), gOpt.Concurrency)); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.Trace(err)
	}

	log.Infof("Reloaded cluster `%s` successfully", name)

	return nil
}
