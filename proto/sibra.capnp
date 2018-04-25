@0xd7ac72be29310d11;
using Go = import "go.capnp";
$Go.package("proto");
$Go.import("github.com/scionproto/scion/go/proto");

struct SibraPCBExt {
    id @0 :Data;
    info @1 :Data;
    up @2 :Bool;
    sofs @3 :List(Data);
}

struct SibraPayload {
}

struct SibraExternalPkt {
    rpkt @0 :Data; # contains the raw packet
}

struct SibraInstruct {
    id @0 :Data;
    expTime @1 :UInt64;
}

struct SibraSteadyReq {
    startIA @0 :UInt64;
    endIA @1 :UInt64;
    segID @2 :Data;
    pathType @3 :UInt8;
}

struct SibraSteadyRecs {
    entries @0 :List(SibraBlockMeta);
}

struct SibraSteadyRep {
    req @0 :SibraSteadyReq;
    recs @1 :SibraSteadyRecs;
}

struct SibraSteadyRegRep {
    ack @0 :List(UInt16);
}

struct SibraBlockMeta {
    id @0 :Data;
    block @1 :Data;
    creation @2 :UInt32;
    interfaces @3 :Data;
    signature @4 :Data;
    whiteList @5 :Data;
    mtu @6 :UInt16;
}


struct SibraMgmt {
    union {
        unset @0 :Void;
        sibraExternalPkt @1 :SibraExternalPkt;
        sibraInstruct @2 :SibraInstruct;
        sibraSteadyReq @3 :SibraSteadyReq;
        sibraSteadyRep @4 :SibraSteadyRep;
        sibraSteadyReg @5 :SibraSteadyRecs;
        sibraSteadyRegRep @6 :SibraSteadyRegRep;
        sibraEphemReq @7 :SibraExternalPkt;
        sibraEphemRep @8 :SibraExternalPkt;
    }
}
