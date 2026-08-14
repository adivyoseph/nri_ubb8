# nri_ubb8
GPU to llm engine cpuset mapping

This NRI plugin is only intended for use on UBB-8 based AMD Instict GPU host servers.

Assumes a dual socket host (NUMA with NPS1) and 4 whole GPU instances per node.

During init the NRI will check for NUMA instance count (2) and chiplets per node (assumes 8), requires at least 5.
Numa 0:
Chiplet 0: reserved for kube-system namespace
Chiplets 1-3: are 16 CPU burstable QoS CPU pools
Chiplets 4-7: are reserved, one chiplet worth of CPU's per GPU instance 0-3
Numa 1:
Chiplets 0-3: are 16 CPU burstable QoS CPU pools
Chiplets 4-7: are reserved, one chiplet worth of CPU's per GPU instance 4-7

When a container is started, gpu device is detected and init details matched.
If non gpu container treated as normal cpu only burstable container and bin packed to least used burstable CPU pool (Chiplet)
If gpu container, gpu instance reserved cpuset (Chiplet) is applied after testing for annotation to disable SMT. If SMT is disabled, only the CPU contect for each Core is added to the cpuset applied. Effectively turning SMT off.

For NRI restarts, active LLM engine mappings will be restabish in local state (no updates made).