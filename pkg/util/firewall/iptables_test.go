// Copyright (c) 2016 Pani Networks
// All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.
//
// firewall_test.go contains test cases for firewall.go

package firewall

// Some comments on use of mocking framework in helpers_test.go.

import (
	"errors"
	"net"
	"strings"
	"testing"

	utilexec "github.com/romana/core/pkg/util/exec"
)

// TestNewChains is checking that detectMissingChains correctly detects which
// Romana chains must be created for given NetIf.
func TestNewChains(t *testing.T) {
	// detectMissingChains calls isChainExist which is reading utilexec.FakeExecutor
	// isChainExist doesn't care for output but must receive not nil error
	// otherwise it would decide that chain exist already and skip
	mockExec := &utilexec.FakeExecutor{Output: nil, Error: errors.New("bla"), Commands: nil}

	fw := IPtables{
		os:            mockExec,
		Store:         firewallStore{},
		Environment:   KubernetesEnvironment,
		networkConfig: mockNetworkConfig{},
	}

	fw.makeRules(mockFirewallEndpoint{"eth0", "A", net.ParseIP("127.0.0.1")})
	newChains := fw.detectMissingChains()

	if len(newChains) != 4 {
		t.Error("TestNewChains failed")
	}

	// TODO test case with some chains already exist requires support for
	// stack of output in utilexec.FakeExecutor
}

// TestCreateChains is checking that CreateChains generates correct OS commands
// for iptables to create firewall chains.
func TestCreateChains(t *testing.T) {
	// CreateChains doesn't care for output
	// we only want to analize which command generated by the function
	mockExec := &utilexec.FakeExecutor{}

	fw := IPtables{
		os:            mockExec,
		Store:         firewallStore{},
		Environment:   KubernetesEnvironment,
		networkConfig: mockNetworkConfig{},
	}
	fw.makeRules(mockFirewallEndpoint{"eth0", "A", net.ParseIP("127.0.0.1")})

	_ = fw.CreateChains([]int{0, 1, 2})

	expect := strings.Join([]string{"/sbin/iptables -N ROMANA-T0S0-INPUT",
		"/sbin/iptables -N ROMANA-T0S0-OUTPUT",
		"/sbin/iptables -N ROMANA-T0S0-FORWARD"}, "\n")

	if *mockExec.Commands != expect {
		t.Errorf("Unexpected input from TestCreateChains, expect\n%s, got\n%s", expect, *mockExec.Commands)
	}
}

// TestDivertTraffic is checking that DivertTrafficToRomanaIPtablesChain generates correct commands for
// firewall to divert traffic into Romana chains.
func TestDivertTraffic(t *testing.T) {
	// We need to simulate failure on response from os.exec
	// so isRuleExist would fail and trigger EnsureRule.
	// But because EnsureRule will use same object as
	// response from os.exec we will see error in test logs,
	// it's ok as long as function generates expected set of commands.
	mockExec := &utilexec.FakeExecutor{Error: errors.New("Rule not found")}

	// Initialize database.
	mockStore := makeMockStore()

	fw := IPtables{
		os:            mockExec,
		Store:         mockStore,
		Environment:   KubernetesEnvironment,
		networkConfig: mockNetworkConfig{},
	}
	fw.makeRules(mockFirewallEndpoint{"eth0", "A", net.ParseIP("127.0.0.1")})

	// 0 is a first standard chain - INPUT
	fw.DivertTrafficToRomanaIPtablesChain(fw.chains[InputChainIndex], installDivertRules)

	expect := "/sbin/iptables -C INPUT -i eth0 -j ROMANA-T0S0-INPUT\n/sbin/iptables -A INPUT -i eth0 -j ROMANA-T0S0-INPUT"

	if *mockExec.Commands != expect {
		t.Errorf("Unexpected input from TestDivertTraffic, expect\n%s, got\n%s", expect, *mockExec.Commands)
	}
	t.Log("All good here, don't be afraid if 'Diverting traffic failed' message")
}

// TestCreateDefaultRules is checking that CreateRules generates correct commands to create
// firewall rules.
func TestCreateDefaultRules(t *testing.T) {
	mockExec := &utilexec.FakeExecutor{}
	ip := net.ParseIP("127.0.0.1")

	// Test default rules wit DROP action
	// Initialize database.
	mockStore := makeMockStore()

	fw := IPtables{
		os:            mockExec,
		Store:         mockStore,
		Environment:   KubernetesEnvironment,
		networkConfig: mockNetworkConfig{},
	}
	fw.makeRules(mockFirewallEndpoint{"eth0", "A", ip})

	// 0 is a first standard chain - INPUT
	fw.CreateDefaultRule(0, targetDrop)

	// expect
	expect := strings.Join([]string{"/sbin/iptables -C ROMANA-T0S0-INPUT -j DROP"},
		"\n")

	if *mockExec.Commands != expect {
		t.Errorf("Unexpected input from TestCreateRules, expect\n%s, got\n%s", expect, *mockExec.Commands)
	}

	// Test default rules wit ACCEPT action
	// Re-initialize database.
	mockStore = makeMockStore()

	// Re-initialize exec
	mockExec = &utilexec.FakeExecutor{}

	// Re-initialize IPtables
	fw = IPtables{
		os:            mockExec,
		Store:         mockStore,
		Environment:   KubernetesEnvironment,
		networkConfig: mockNetworkConfig{},
	}
	fw.makeRules(mockFirewallEndpoint{"eth0", "A", ip})

	// 0 is a first standard chain - INPUT
	fw.CreateDefaultRule(0, targetAccept)

	// expect
	expect = strings.Join([]string{"/sbin/iptables -C ROMANA-T0S0-INPUT -j ACCEPT"}, "\n")

	if *mockExec.Commands != expect {
		t.Errorf("Unexpected input from TestCreateRules, expect\n%s, got\n%s", expect, *mockExec.Commands)
	}

}

// TestCreateRules is checking that CreateRules generates correct commands to create
// firewall rules.
func TestCreateRules(t *testing.T) {
	// we only care for recorded commands, no need for fake output or errors
	mockExec := &utilexec.FakeExecutor{}

	// Initialize database.
	mockStore := makeMockStore()

	fw := IPtables{
		os:            mockExec,
		Store:         mockStore,
		Environment:   KubernetesEnvironment,
		networkConfig: mockNetworkConfig{},
	}
	fw.Init(mockFirewallEndpoint{"eth0", "A", net.ParseIP("127.0.0.1")})

	rule := NewFirewallRule()
	rule.SetBody("ROMANA-T0S0-INPUT -d 255.255.255.255/32 -p udp -m udp --sport 68 --dport 67 -j ACCEPT")
	rules := []FirewallRule{rule}

	fw.SetDefaultRules(rules)

	//	fw.chains[inputChainIndex].Rules =
	// 0 is a first standard chain - INPUT
	fw.CreateRules(InputChainIndex)

	expect := strings.Join([]string{
		"/sbin/iptables -C ROMANA-T0S0-INPUT -d 255.255.255.255/32 -p udp -m udp --sport 68 --dport 67 -j ACCEPT",
	}, "\n")

	if *mockExec.Commands != expect {
		t.Errorf("Unexpected input from TestCreateRules, expect\n%s, got\n%s", expect, *mockExec.Commands)
	}
}

// TestCreateU32Rule is checking that CreateU32Rules generates correct commands to
// create firewall rules.
func TestCreateU32Rules(t *testing.T) {

	// we only care for recorded commands, no need for fake output or errors
	mockExec := &utilexec.FakeExecutor{}

	// Initialize database.
	mockStore := makeMockStore()

	fw := IPtables{
		os:            mockExec,
		Store:         mockStore,
		Environment:   KubernetesEnvironment,
		networkConfig: mockNetworkConfig{},
	}
	fw.makeRules(mockFirewallEndpoint{"eth0", "A", net.ParseIP("127.0.0.1")})

	// 0 is a first standard chain - INPUT
	fw.CreateU32Rules(0)

	expect := strings.Join([]string{"/sbin/iptables -A ROMANA-T0S0-INPUT -m u32 --u32 12&0xFF00FF00=0x7F000000 && 16&0xFF00FF00=0x7F000000 -j ACCEPT"}, "\n")

	if *mockExec.Commands != expect {
		t.Errorf("Unexpected input from TestCreateU32Rules, expect\n%s, got\n%s", expect, *mockExec.Commands)
	}
}
