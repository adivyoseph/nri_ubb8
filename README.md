# nri_ubb8
GPU to llm engine cpuset mapping on AMD UBB-8 Host systems

This NRI plugin is only intended for use on UBB-8 based AMD Instict GPU host servers.
This a POC to allow testing of performance differences between standard Kubernetes and topology aware bin packing (CCX's).

Assumes a dual socket host (NUMA with NPS1) and 4 whole GPU instances per node. (tested on single socket two CCX system)

During init the NRI will check for NUMA instance count (2) and chiplets per node (assumes 8), requires at least 5.
Numa 0:
Chiplet 0: reserved for kube-system namespace. requires coordination with kubeletconfig. No checks currently in the code.
Chiplets 1-3: are 16 CPU burstable QoS CPU pools, 3 of
Chiplets 4-7: are reserved, one chiplet worth of CPU's per GPU instance 0-3
Numa 1:
Chiplets 0-3: are 16 CPU burstable QoS CPU pools, 4 of
Chiplets 4-7: are reserved, one chiplet worth of CPU's per GPU instance 4-7

When a container is started, gpu device is detected and init details matched.
If non gpu container treated as normal cpu only burstable container and bin packed to least used burstable CPU pool (Chiplet). For now cpu quata asks that exceed a CCX are attemped to be fit into a single (whole) ccx.
If gpu container, gpu instance reserved cpuset (Chiplet) is applied after testing for annotation to disable SMT. If SMT is disabled, only the first CPU contect for each Core is added to the cpuset applied. Effectively turning SMT off (see notes below). The Container Spec vcpu (quanta) ask is ignored, a whole chiplet (CCX0 wort of CPU's is applied always.)

For NRI restarts, active LLM engine mappings will be re-estabish in local state (no updates made). Policy on new PODs only.


POD annotations required
"amd.llm"  val == "yes" will map linux devices added by CDI as /dev/dri card1 through n, they get their own ccx's
"amd.smt"  vall=="off"  will reduce ccx cpuset asigned to a vllm engine to just be the cpu contexts whn BIOS smt is enabled, only lasts for the life of the container


TODO
add kubeletconfig checks
test with tensor parallel that all card instances get mapped to a single CCX (may be a problem if > than 4 due to numa)
Assume that p/d vllm instances are treated as tensor parallel == 1
see if there is a better way to detect a llm-engine container



manual build and test
in a seperate console
sudo go run cmd/main.go

ctrl C to terminate, no artifacts left

all logging will be on the console