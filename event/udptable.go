package event

import (
	"fmt"
	"net"
	"net/netip"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	_ "unsafe"
)

type udpListener struct {
	mu                sync.RWMutex
	currentWalkStatue bool

	pidMap map[uint32]struct{}
	udpMap map[udpKey]*udpElem

	t udpTable
}

func NewUDPListener() *udpListener {
	return &udpListener{
		pidMap: map[uint32]struct{}{},
		udpMap: map[udpKey]*udpElem{},
		t:      make(udpTable, 512),
	}
}

type udpKey struct {
	addr netip.AddrPort
	pid  uint32
}

type udpElem struct {
	walkStatue bool
}

type UDPEvent struct {
	LocAddr netip.AddrPort
	Pid     uint32
	start   bool
}

func (e *UDPEvent) Start() bool { return e.start }

func (u *udpListener) Register(pid uint32) *udpListener {
	u.mu.Lock()
	u.pidMap[pid] = struct{}{}
	u.mu.Unlock()
	return u
}

func (u *udpListener) Delete(pid uint32) *udpListener {
	u.mu.Lock()
	delete(u.pidMap, pid)
	u.mu.Unlock()
	return u
}

func (u *udpListener) Walk() (ues []UDPEvent, err error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if len(u.pidMap) == 0 {
		return ues, nil
	}

	if u.t, err = getExtendedUdpTableWithPid(u.t); err != nil {
		return ues, err
	}

	u.currentWalkStatue = !u.currentWalkStatue
	n := int(u.t.Entries())
	for i := 0; i < n; i++ {
		if pid := u.t.Pid(i); pid > 4 {
			if _, has := u.pidMap[pid]; has {
				if e, has := u.udpMap[udpKey{u.t.AddrPort(i), pid}]; has {
					e.walkStatue = u.currentWalkStatue
				} else {
					u.udpMap[udpKey{u.t.AddrPort(i), pid}] = &udpElem{
						walkStatue: u.currentWalkStatue,
					}
					ues = append(ues, UDPEvent{LocAddr: u.t.AddrPort(i), Pid: pid, start: true})
				}
			}
		}
	}

	for k, v := range u.udpMap {
		if v.walkStatue != u.currentWalkStatue {
			delete(u.udpMap, k)
			ues = append(ues, UDPEvent{LocAddr: k.addr, Pid: k.pid, start: false})
		}
	}

	return ues, nil
}

//go:linkname modiphlpapi golang.org/x/sys/windows.modiphlpapi
var modiphlpapi *windows.LazyDLL

var procGetExtendedUdpTable = modiphlpapi.NewProc("GetExtendedUdpTable")

type udpTable []byte

func (u udpTable) Entries() uint32 {
	return uint32(len(u) / 12)
}

func (u udpTable) grow(to int) udpTable {
	for len(u) < to {
		u = append(u, 0)
		u = u[:cap(u)]
	}
	return u
}

func (u udpTable) Addr(idx int) (ip net.IP) {
	// type row struct {
	// 	dwLocalAddr uint32
	// 	dwLocalPort uint32
	// 	dwOwningPid uint32
	// }

	s := 4 + idx*12 + 0
	e := s + 4
	return net.IP(u[s:e])
}

func (u udpTable) Port(idx int) uint16 {
	return uint16(*(*uint32)(unsafe.Pointer((&u[4+idx*12+4]))))
}

func (u udpTable) AddrPort(idx int) netip.AddrPort {
	return netip.AddrPortFrom(
		netip.AddrFrom4([4]byte(u.Addr(idx))),
		u.Port(idx),
	)
}

func (u udpTable) Pid(idx int) uint32 {

	return *(*uint32)(unsafe.Pointer((&u[4+idx*12+8])))
}

func getExtendedUdpTableWithPid(b udpTable) (udpTable, error) {
	type UDP_TABLE_CLASS int
	const (
		UDP_TABLE_BASIC UDP_TABLE_CLASS = iota
		UDP_TABLE_OWNER_PID
		UDP_TABLE_OWNER_MODULE
	)

start:
	var size uint32 = uint32(len(b))
	var pTable uintptr = 0
	if len(b) > 0 {
		pTable = uintptr(unsafe.Pointer(&b[0]))
	}

	_, _, e1 := syscall.SyscallN(
		procGetExtendedUdpTable.Addr(),
		pTable,
		uintptr(unsafe.Pointer(&size)),
		0, // false
		windows.AF_INET,
		uintptr(UDP_TABLE_OWNER_PID),
		0,
	)
	if e1 == windows.ERROR_INSUFFICIENT_BUFFER ||
		len(b) < int(size) {

		b = b.grow(int(size))
		goto start
	}

	if e1 == 0 {
		if b.Entries()*12+4 > uint32(len(b)) {
			panic(fmt.Sprintf("impossible entries:%d len: %d", b.Entries(), uint32(len(b))))
		}
		return b, nil
	} else {
		return b, e1
	}
}
