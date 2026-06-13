// SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
	"time"

	mrand "math/rand"
)

func generateMacAddress() string {
	buf := make([]byte, 6)
	rand.Read(buf)

	// Locally administered, unicast.
	buf[0] |= 2
	buf[0] &^= 1
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", buf[0], buf[1], buf[2], buf[3], buf[4], buf[5])
}

func generateInteger(max int) int {
	s := mrand.NewSource(time.Now().UnixNano())
	r := mrand.New(s)

	return r.Intn(max)
}

func generateIPAddress() string {
	buf := make([]byte, 4)

	ip := mrand.Uint32()
	binary.BigEndian.PutUint32(buf, ip)

	return net.IP(buf).String()
}

// generateIPAddressInCIDR returns a random host address within the given
// IPv4 CIDR. Network and broadcast addresses are skipped.
func generateIPAddressInCIDR(cidr string) (string, error) {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", err
	}
	ip4 := ipnet.IP.To4()
	if ip4 == nil {
		return "", fmt.Errorf("only IPv4 CIDRs are supported: %q", cidr)
	}
	ones, _ := ipnet.Mask.Size()
	hostBits := 32 - ones
	if hostBits == 0 {
		return ip4.String(), nil
	}
	var maxHost uint32
	if hostBits >= 32 {
		maxHost = ^uint32(0) - 2
	} else {
		maxHost = (uint32(1) << hostBits) - 2
	}
	offset := mrand.Uint32()%maxHost + 1

	base := binary.BigEndian.Uint32(ip4)
	out := make(net.IP, 4)
	binary.BigEndian.PutUint32(out, base+offset)
	return out.String(), nil
}

func getStrPtr(s string) *string {
	sp := s
	return &sp
}
