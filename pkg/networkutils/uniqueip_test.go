package networkutils_test

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/aws/eks-anywhere/pkg/networkutils"
)

type DummyNetClient struct{}

func (n *DummyNetClient) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	// add dummy case for coverage
	if address == "255.255.255.255:22" {
		return &net.IPConn{}, nil
	}
	return nil, errors.New("")
}

func TestGenerateUniqueIP(t *testing.T) {
	cidrBlock := "1.2.3.4/16"

	ipgen := networkutils.NewIPGenerator(&DummyNetClient{})
	ip, err := ipgen.GenerateUniqueIP(cidrBlock)
	if err != nil {
		t.Fatalf("GenerateUniqueIP() ip = %v error: %v", ip, err)
	}
}

func TestIsIPUniquePass(t *testing.T) {
	ip := "0.0.0.0"

	ipgen := networkutils.NewIPGenerator(&DummyNetClient{})
	if !ipgen.IsIPUnique(ip) {
		t.Fatalf("Expected IP: %s to be unique but it is not", ip)
	}
}

func TestIsIPUniqueFail(t *testing.T) {
	ip := "255.255.255.255"

	ipgen := networkutils.NewIPGenerator(&DummyNetClient{})
	if ipgen.IsIPUnique(ip) {
		t.Fatalf("Expected IP: %s to be not be unique but it is", ip)
	}
}
