package main

import (
	"log"

	"github.com/adivyoseph/nri_ubb8/pkg/allocate"
	//plugin "github.com/adivyoseph/nri_ubb8/pkg/plugin"
	"github.com/adivyoseph/nri_ubb8/pkg/topology"
)

func main() {

	log.SetPrefix("AMD_ubb8_nri")
	log.Printf("AMD UBB8 NRI plugin starting\n")

	//Todo add system check to make sure that it is a UBB8 host

	//get the host topology and build GPU instance lookup table

	topo, err := topology.GetTopology()
	if err != nil {
		log.Fatal(-1)
	}
	//debug
	log.Printf("Topology: cpus %d, smtOn %t\n", topo.Cpus_total, topo.SmtOn)

	//No config file required initially
	//all features driven from container annotations

	//Initialize resource allocator
	allocate.Init(topo)

	//Todo check kubelet for reserved core cpuset
	//For now reserve node_0::ccx_0

	log.Printf("plugin.PluginStart\n")

	//go plugin.PluginStart()
	//Todo impliment systemd or deamon loading
	for {
	}

}
