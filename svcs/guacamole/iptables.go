package guacamole

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"

	"github.com/rs/zerolog/log"
)

type IPTables struct {
	sudo bool

	// flags to service
	flags []string

	// enable debug or not
	debug bool

	// Implementation of ExecFunc.
	execFunc ExecFunc

	// Implementation of PipeFunc.
	pipeFunc PipeFunc
}

var (
	cmd = "iptables"
	//Name of the chaines
	inputC   = Chain("INPUT")
	forwardC = Chain("FORWARD")
	outputC  = Chain("OUTPUT")

	//name of the policy
	acceptP = Policy("ACCEPT")
	dropP   = Policy("DROP")
	rejectP = Policy("REJECT")
	returnP = Policy("RETURN")

	deleteA = Action("-D")       // delete action
	insertA = Action("--insert") // insert action

)

type Action string

type Chain string

type Policy string

type PipeFunc func(stdin io.Reader, cmd string, args ...string) ([]byte, error)

type ExecFunc func(cmd string, args ...string) ([]byte, error)

type Errori struct {
	Out []byte
	Err error
}

// iptables --insert DOCKER-USER -s 77.179.248.0/24 -j REJECT --reject-with icmp-port-unreachable
func (ipTab *IPTables) createRejectRule(labSubnet string) error {
	log.Debug().Msgf("Reject icmp connection on containers for lab %s", labSubnet)
	cmds := []string{string(insertA), "DOCKER-USER", "-s", labSubnet, "-j", string(rejectP), "--reject-with", "icmp-port-unreachable"}
	_, err := ipTab.execute(cmds...)
	return err
}

// sudo iptables --insert DOCKER-USER -s 77.218.127.0/24 -m state --state RELATED,ESTABLISHED -j RETURN
func (ipTab *IPTables) createStateRule(labSubnet string) error {
	cmds := []string{string(insertA), "DOCKER-USER", "-s", labSubnet, "-m", "state", "--state", "RELATED,ESTABLISHED", "-j", string(returnP)}
	log.Debug().Strs("STATE RULE", cmds).Msgf("Creating Iptables State Rule for VPN Connection")
	_, err := ipTab.execute(cmds...)
	return err
}

// iptables --insert DOCKER-USER -s 77.179.248.0/24 -d 25.136.240.250/32,25.136.241.249/32,25.136.242.248/32,25.136.243.247/32,77.179.248.0/24 -j ACCEPT
func (ipTab *IPTables) createAcceptRule(labSubnet string, vpnIPs string) error {
	cmds := []string{string(insertA), "DOCKER-USER", "-s", labSubnet, "-d", vpnIPs, "-j", string(acceptP)}
	_, err := ipTab.execute(cmds...)
	return err
}

func (ipTab *IPTables) removeAcceptRule(labSubnet string, vpnIps string) error {
	cmds := []string{string(deleteA), "DOCKER-USER", "-s", labSubnet, "-d", vpnIps, "-j", string(acceptP)}
	log.Debug().Strs("ACCEPT RULE", cmds).Msgf("Removing Iptables Accept Rule for VPN Connection")
	_, err := ipTab.execute(cmds...)
	return err
}
func (ipTab *IPTables) removeStateRule(labSubnet string) error {
	cmds := []string{string(deleteA), "DOCKER-USER", "-s", labSubnet, "-m", "state", "--state", "RELATED,ESTABLISHED", "-j", string(returnP)}
	log.Debug().Strs("STATE RULE", cmds).Msgf("Removing Iptables State Rule for VPN Connection")
	_, err := ipTab.execute(cmds...)
	return err
}

func (ipTab *IPTables) removeRejectRule(labSubnet string) error {
	cmds := []string{string(deleteA), "DOCKER-USER", "-s", labSubnet, "-j", string(rejectP), "--reject-with", "icmp-port-unreachable"}
	log.Debug().Strs("Reject RULE", cmds).Msgf("Removing Iptables Reject Rule for VPN Connection")
	_, err := ipTab.execute(cmds...)
	return err
}

func (e Errori) Error() string {
	return fmt.Sprintf("%s: %s", e.Err, string(e.Out))
}

// exec executes an ExecFunc using 'iptables'.
func (ipTab *IPTables) execute(args ...string) ([]byte, error) {
	return ipTab.exec(cmd, args...)
}

// exec executes an ExecFunc using 'iptables'.
func (ipTab *IPTables) exec(cmd string, args ...string) ([]byte, error) {
	flags := append(ipTab.flags, args...)

	// If needed, prefix sudo.
	if ipTab.sudo {
		flags = append([]string{cmd}, flags...)
		cmd = "sudo"
	}
	log.Debug().Msgf("exec %s %v", cmd, flags)
	out, err := ipTab.execFunc(cmd, flags...)
	if out != nil {
		out = bytes.TrimSpace(out)
		log.Debug().Msgf("exec: %q", string(out))
	}
	if err != nil {
		// Wrap errors in Error type for further introspection
		return nil, &Errori{
			Out: out,
			Err: err,
		}
	}
	return out, nil
}
func shellExec(cmd string, args ...string) ([]byte, error) {
	return exec.Command(cmd, args...).CombinedOutput()
}
