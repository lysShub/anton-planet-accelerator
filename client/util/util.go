package util

import (
	"context"
	"fmt"
	"math"
	"net/netip"
	"syscall"
	"time"
	"unsafe"

	"github.com/lxn/win"
	"github.com/shirou/gopsutil/process"
	"golang.org/x/sys/windows"
)

func GetWarThunderPid(ctx context.Context) (int, error) {
	const warThunderName = "WarThunder.exe"

	for {
		ps, err := process.Processes()
		if err != nil {
			return 0, err
		}

		for _, p := range ps {
			if n, err := p.Name(); err != nil {
				continue
			} else if n == warThunderName {
				return int(p.Pid), nil
			}
		}

		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}

		time.Sleep(time.Millisecond * 250)
	}
}

const TCPIP_OWNING_MODULE_SIZE = 16

type MIB_UDPROW_OWNER_MODULE struct {
	dwLocalAddr       uint32
	dwLocalPort       uint32
	dwOwningPid       uint32
	liCreateTimestamp int64 // because of the alignment, this field type can't be win.FILETIME

	specificPortBind int32
	owningModuleInfo [TCPIP_OWNING_MODULE_SIZE]uint64
}

func (r *MIB_UDPROW_OWNER_MODULE) LocalAddr() netip.Addr {
	return netip.AddrFrom4(*(*[4]byte)(unsafe.Pointer(&r.dwLocalAddr)))
}

func (r *MIB_UDPROW_OWNER_MODULE) LocalPort() int {
	return int(r.dwLocalPort)
}

func (r *MIB_UDPROW_OWNER_MODULE) OwningPid() int {
	return int(r.dwOwningPid)
}

// CreateTime UTC-time
func (r *MIB_UDPROW_OWNER_MODULE) CreateTime() time.Time {
	var f = &win.SYSTEMTIME{}
	if ok := win.FileTimeToSystemTime((*win.FILETIME)(unsafe.Pointer(&r.liCreateTimestamp)), f); !ok {
		return time.Time{}
	}

	return time.Date(
		int(f.WYear), time.Month(f.WMonth), int(f.WDay),
		int(f.WHour), int(f.WMinute), int(f.WSecond),
		int(f.WMilliseconds)*int(time.Millisecond), time.UTC)
}

func (r *MIB_UDPROW_OWNER_MODULE) Connected() bool {
	return r.specificPortBind != 1
}

type MIB_UDPTABLE_OWNER_MODULE struct {
	dwNumEntries uint32
	table        [0]MIB_UDPROW_OWNER_MODULE
}

func (t *MIB_UDPTABLE_OWNER_MODULE) GetTable(idx int) *MIB_UDPROW_OWNER_MODULE {
	const size = unsafe.Sizeof(MIB_UDPROW_OWNER_MODULE{})

	if idx < 0 || idx >= int(t.dwNumEntries) {
		return nil
	}
	p := unsafe.Pointer(&t.table)
	return (*MIB_UDPROW_OWNER_MODULE)(unsafe.Add(p, uintptr(idx)*size))
}

func newUDPTable(byte int) *MIB_UDPTABLE_OWNER_MODULE {
	const size = unsafe.Sizeof(MIB_UDPROW_OWNER_MODULE{})
	n := int(math.Ceil(float64(byte-4) / float64(size)))
	if n < 1 {
		return nil
	}

	var _t = make([]MIB_UDPROW_OWNER_MODULE, n)
	return &MIB_UDPTABLE_OWNER_MODULE{
		dwNumEntries: uint32(n),
		table:        *(*[0]MIB_UDPROW_OWNER_MODULE)(unsafe.Pointer(&_t[0])),
	}
}

var iphlpapi = windows.NewLazySystemDLL("iphlpapi.dll")
var getExtendedUdpTableProc = iphlpapi.NewProc("GetExtendedUdpTable")

// TODO: support IPv6
func GetExtendedUdpTable() (*MIB_UDPTABLE_OWNER_MODULE, error) {
	type UDP_TABLE_CLASS int
	const (
		UDP_TABLE_BASIC UDP_TABLE_CLASS = iota
		UDP_TABLE_OWNER_PID
		UDP_TABLE_OWNER_MODULE
	)

	// ----------------------------------------------------------
	var byte int32 = 512
	var count int = 0
start:
	t := newUDPTable(int(byte))
	r1, _, err := getExtendedUdpTableProc.Call(
		uintptr(unsafe.Pointer(t)),
		uintptr(unsafe.Pointer(&byte)),
		uintptr(1),
		uintptr(syscall.AF_INET),
		uintptr(UDP_TABLE_OWNER_MODULE),
		0,
	)
	if count > 8 {
		return nil, fmt.Errorf("GetExtendedUdpTable failed")
	} else if r1 == uintptr(windows.ERROR_INSUFFICIENT_BUFFER) {
		byte += 64
		count++
		goto start
	}
	if err != windows.NOERROR {
		return nil, err
	}

	return t, nil
}

func GetUDPTableByPid(pid int) ([]MIB_UDPROW_OWNER_MODULE, error) {
	t, err := GetExtendedUdpTable()
	if err != nil {
		return nil, err
	}

	var rows []MIB_UDPROW_OWNER_MODULE
	for i := 0; i < int(t.dwNumEntries); i++ {
		row := t.GetTable(i)
		if row.OwningPid() == pid {
			rows = append(rows, *row)
		}
	}
	return rows, nil
}
