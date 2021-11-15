package path

import (
	"github.com/scionproto/scion/go/lib/slayers"
	"github.com/scionproto/scion/go/lib/slayers/path"
	"github.com/scionproto/scion/go/lib/slayers/path/onehop"
)

type OneHop struct {
	Info     path.InfoField
	FirstHop path.HopField
}

func (p OneHop) SetPath(s *slayers.SCION) error {
	ohp := &onehop.Path{
		Info:     p.Info,
		FirstHop: p.FirstHop,
	}
	s.Path, s.PathType = ohp, ohp.Type()
	return nil
}
