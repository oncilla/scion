// Copyright 2018 ETH Zurich
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

import (
	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/hpkt"
	"github.com/scionproto/scion/go/lib/sibra/sbextn"
	"github.com/scionproto/scion/go/lib/sock/reliable"
	"github.com/scionproto/scion/go/lib/spkt"
	"github.com/scionproto/scion/go/lib/topology"
	"github.com/scionproto/scion/go/sibra_srv/conf"
	"github.com/scionproto/scion/go/sibra_srv/sbalgo"
)

func Forward(pkt *conf.ExtPkt) error {
	var base *sbextn.Base
	switch {
	case pkt.Steady != nil:
		base = pkt.Steady.Base
	case pkt.Ephem != nil:
		base = pkt.Ephem.Base
	default:
		return common.NewBasicError("Unable to forward. No SIBRA extension present", nil)
	}
	if base.IsTransfer() {
		if err := base.NextSOFIndex(); err != nil {
			return err
		}
	}
	ifids, err := GetPktIfids(base, pkt.Spkt)
	if err != nil {
		return err
	}
	dst, err := GetAddrForIFID(ifids.EgIfid, pkt.Conf.Topo)
	if err != nil {
		return err
	}
	// In steady setup packets, the border router does not update the SOFIndex
	if base.Setup {
		if err := base.NextSOFIndex(); err != nil {
			return err
		}
	}
	buf, err := PackWithPld(pkt.Spkt, pkt.Pld)
	if err != nil {
		return err
	}
	_, err = pkt.Conf.Conn.WriteTo(buf, dst)
	return err
}

func PackWithPld(pkt *spkt.ScnPkt, pld common.Payload) (common.RawBytes, error) {
	pkt.Pld = pld
	buf := make(common.RawBytes, pkt.TotalLen())
	n, err := hpkt.WriteScnPkt(pkt, buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

func GetAddrForIFID(ifid common.IFIDType, topo *topology.Topo) (*reliable.AppAddr, error) {
	ifinfo, ok := topo.IFInfoMap[ifid]
	if !ok {
		return nil, common.NewBasicError("Interface not found", nil, "IFID", ifid)
	}
	nextAI := ifinfo.InternalAddr.PublicAddrInfo(topo.Overlay)
	a := &reliable.AppAddr{
		Addr: addr.HostFromIP(nextAI.IP),
		Port: uint16(nextAI.L4Port),
	}
	return a, nil
}

func GetSibraExtn(pkt *spkt.ScnPkt) (common.ExtnBase, error) {
	for _, hbh := range pkt.HBHExt {
		if hbh.Type() == common.ExtnSIBRAType {
			return hbh, nil
		}
	}
	return nil, common.NewBasicError("No SIBRA extension found", nil)
}

// GetResvIfids returns the interfaces in the direction of the reservation.
// This is independent of the direction of travel of the packet.
func GetResvIfids(base *sbextn.Base, pkt *spkt.ScnPkt) (sbalgo.IFTuple, error) {
	ifids, err := GetPktIfids(base, pkt)
	if !base.Forward {
		return ifids.Reverse(), err
	}
	return ifids, err
}

// GetPktIfids returns the interfaces in the direction of travel of the packet.
// InIfid corresponds to the interface where the packet entered the AS.
// EgIfid where it leaves the AS.
func GetPktIfids(b *sbextn.Base, pkt *spkt.ScnPkt) (sbalgo.IFTuple, error) {
	if b.Setup {
		return getPktIfidsSetup(pkt)
	}
	return getPktIfids(b)
}

func getPktIfidsSetup(pkt *spkt.ScnPkt) (sbalgo.IFTuple, error) {
	if pkt.Path == nil {
		return sbalgo.IFTuple{}, common.NewBasicError("Path is nil", nil)
	}
	info, err := pkt.Path.GetInfoField(pkt.Path.InfOff)
	if err != nil {
		return sbalgo.IFTuple{}, err
	}
	hopF, err := pkt.Path.GetHopField(pkt.Path.HopOff)
	if err != nil {
		return sbalgo.IFTuple{}, err
	}
	ifids := sbalgo.IFTuple{
		InIfid: hopF.ConsIngress,
		EgIfid: hopF.ConsEgress,
	}
	if !info.ConsDir {
		return ifids.Reverse(), nil
	}
	return ifids, nil
}

func getPktIfids(ext *sbextn.Base) (sbalgo.IFTuple, error) {
	sof := ext.GetCurrBlock().SOFields[ext.RelSOFIdx]
	ifids := sbalgo.IFTuple{
		InIfid: sof.Ingress,
		EgIfid: sof.Egress,
	}
	if !ext.Forward {
		return ifids.Reverse(), nil
	}
	return ifids, nil
}
