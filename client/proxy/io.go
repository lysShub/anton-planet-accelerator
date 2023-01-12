package proxy

type Io interface {
	Read(p *Upack) (err error)
	Write(p *Upack) (err error)
}
