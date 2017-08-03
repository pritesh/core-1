package main

import (
	"fmt"
	"net"
	"syscall"

	log "github.com/romana/rlog"
	"github.com/vishvananda/netlink"
)

var (
	kernelDefaults = []string{
		"/proc/sys/net/ipv4/conf/default/proxy_arp",
		"/proc/sys/net/ipv4/conf/all/proxy_arp",
		"/proc/sys/net/ipv4/ip_forward",
	}
)

func CreateRomanaGW() error {
	rgw := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: "romana-lo", TxQLen: 1000}}
	if err := netlink.LinkAdd(rgw); err != nil {
		if err == syscall.EEXIST {
			log.Warn("Romana gateway already exists.")
		} else {
			log.Info("Error adding Romana gateway to node:", err)
			return err
		}
	} else {
		log.Info("Successfully added romana gateway to node.")
	}

	if err := netlink.LinkSetUp(rgw); err != nil {
		log.Error("Error while brining up romana gateway:", err)
		return err
	}

	return nil
}

func SetRomanaGwIP(romanaIp string) error {
	nip := net.ParseIP(romanaIp)
	if nip == nil {
		return fmt.Errorf("Failed to parse ip address %s", romanaIp)
	}

	ipnet := &net.IPNet{IP: nip, Mask: net.IPMask([]byte{0xff, 0xff, 0xff, 0xff})}

	link, err := netlink.LinkByName("romana-lo")
	if err != nil {
		return err
	}

	addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		return err
	}

	for _, addr := range addrs {
		if addr.IPNet.String() == ipnet.String() {
			log.Debugf("Address %s already installed in interface %s", addr, link)
			return nil
		}
	}

	ip := &netlink.Addr{
		IPNet: ipnet,
	}

	err = netlink.AddrAdd(link, ip)
	if err != nil {
		return err
	}

	return nil
}