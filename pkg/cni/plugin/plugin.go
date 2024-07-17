/*
 * This file is part of the KubeVirt project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright 2023 Red Hat, Inc.
 *
 */

package plugin

import (
	"fmt"
	"log"

	vishnetlink "github.com/vishvananda/netlink"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	type100 "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/plugins/pkg/ns"

	"github.com/qinqon/kubevirt-passt-binding/pkg/cni/plugin/netlink"
	"github.com/qinqon/kubevirt-passt-binding/pkg/cni/plugin/sysctl"

	"github.com/qinqon/kubevirt-passt-binding/pkg/link"
)

const (
	virtLauncherUserID = 107
)

func CmdAdd(args *skel.CmdArgs) error {
	netns, err := ns.GetNS(args.Netns)
	if err != nil {
		return fmt.Errorf("failed to open netns %q: %v", netns, err)
	}
	defer netns.Close()

	c := NewCmd(netns, sysctl.New(), netlink.New())
	result, err := c.CmdAddResult(args)
	if err != nil {
		return err
	}
	return result.Print()
}

func CmdDel(args *skel.CmdArgs) error {
	return nil
}

func CmdCheck(args *skel.CmdArgs) error {
	return nil
}

type sysctlAdapter interface {
	IPv4SetUnprivilegedPortStart(int) error
	IPv4SetPingGroupRange(int, int) error
}

type netlinkAdapter interface {
	ReadLink(string) (vishnetlink.Link, error)
}

type cmd struct {
	netns          ns.NetNS
	sysctlAdapter  sysctlAdapter
	netlinkAdapter netlinkAdapter
}

func NewCmd(netns ns.NetNS, sysctlAdapter sysctlAdapter, netlinkAdapter netlinkAdapter) *cmd {
	return &cmd{netns: netns, sysctlAdapter: sysctlAdapter, netlinkAdapter: netlinkAdapter}
}

func (c *cmd) CmdAddResult(args *skel.CmdArgs) (types.Result, error) {
	netConf, cniVersion, err := loadConf(args.StdinData)
	if err != nil {
		return nil, err
	}

	result := type100.Result{CNIVersion: cniVersion}

	err = c.netns.Do(func(_ ns.NetNS) error {
		if err := c.sysctlAdapter.IPv4SetUnprivilegedPortStart(0); err != nil {
			return err
		}
		if err := c.sysctlAdapter.IPv4SetPingGroupRange(virtLauncherUserID, virtLauncherUserID); err != nil {
			return err
		}

		netname := netConf.Args.Cni.LogicNetworkName
		log.Printf("setup for logical network %s completed successfully", netname)

		defaultGatwayLinks, err := link.DiscoverByDefaultGateway(vishnetlink.FAMILY_ALL)
		if err != nil {
			return err
		}

		if len(defaultGatwayLinks) != 1 {
			return fmt.Errorf("unexpected number of default gw links")
		}

		podLink := defaultGatwayLinks[0]

		addrs, err := vishnetlink.AddrList(podLink, vishnetlink.FAMILY_ALL)
		if err != nil {
			return err
		}

		dummyLink := &vishnetlink.Dummy{LinkAttrs: vishnetlink.LinkAttrs{
			Name: args.IfName,
		}}
		if err := vishnetlink.LinkAdd(dummyLink); err != nil {
			return err
		}

		result.Interfaces = append(result.Interfaces, &type100.Interface{
			Name:    podLink.Attrs().Name,
			Mac:     podLink.Attrs().HardwareAddr.String(),
			Sandbox: c.netns.Path(),
		})
		for _, addr := range addrs {
			result.IPs = append(result.IPs, &type100.IPConfig{
				Address: *addr.IPNet,
			})
			addr.Label = ""
			if err := vishnetlink.AddrAdd(dummyLink, &addr); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &result, nil
}
