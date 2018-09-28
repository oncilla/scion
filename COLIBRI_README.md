# COLIBRI 

This is a prototype implementation of a Cooperative Lightweight Inter-domain 
Bandwidth Reservation Infrastructure (COLIBRI). The codename throughout the 
code base is currently SIBRA, as it is the conceptual ancestor. To test
COLIBRI, we assume that SCION has already been set up properly (if not, refer
to the [readme](README.md)). 

To generate the topology, run the following command:

```
./scion.sh topology -sibra
```

Run the infrastructure with following command:
```
./scion.sh run
```

This will start the COLIBRI services, which automatically establishes
the reservations specified in the configuration dir 
(`confdir/sibra/reservations.json`).
By default these are one Up and one Down reservation to/from the issuer
for non-core ASes. For core ASes one reservation to every other core
is established.

An example application using the reservation manager and establishing 
an ephemeral path can be found in [sibra_rpt](go/examples/sibra_rpt/pingpong.go)
or [sibra_quic](go/examples/sibra_quic/pingpong.go). It is a simple
client server pingpong example.

## Code Overview

* [Library](go/lib/sibra) contains the definitions, extension headers and request
 payloads.
* [COLIBRI Server](go/sibra_srv) contains the server implementation.
* [COLIBRI Daemon](go/sibrad) contains the reservaiton manager.


## Future Work
* Support peering links and telescoping
* Extract possible paths from beacons instead of fetching path from path manager
* The COLIBRI daemon should be integrated into sciond or be a separate entity.
 Running one daemon per application is not possible, since COLIBRI specifies a 
 port that the reservation manager listens on. 
* Support reservation registration, also including whitelisting (currently the 
 reservation is directly fetched from the reservation owner)
* Adaptive ephemeral traffic bound. Currently, the transfer ASes simply 
 accept ephemeral reservations in a greedy manner. They should respect the 
 minimal guarantees and fluidly reshape the ephemeral reservations.
* Client rate limiting (both request and bandwidth) at end host and edge AS 
 border router.
* Probabilistic and per-flow stateless monitoring in non-edge ASes.
