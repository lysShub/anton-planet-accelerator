//go:build windows
// +build windows

package divert

import (
	"net/netip"
	"reflect"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	DLLPath = `D:\OneDrive\code\go\安东星加速器\core\WinDivert.dll`
)

// TODO: sync.Once on Open
var divert *windows.DLL = windows.MustLoadDLL(DLLPath)
var (
	winDivertOpenProc     = divert.MustFindProc("WinDivertOpen")
	winDivertRecvProc     = divert.MustFindProc("WinDivertRecv")
	winDivertRecvExProc   = divert.MustFindProc("WinDivertRecvEx")
	winDivertSendProc     = divert.MustFindProc("WinDivertSend")
	winDivertSendExProc   = divert.MustFindProc("WinDivertSendEx")
	winDivertShutdownProc = divert.MustFindProc("WinDivertShutdown")
	winDivertCloseProc    = divert.MustFindProc("WinDivertClose")
	winDivertSetParamProc = divert.MustFindProc("WinDivertSetParam")
	winDivertGetParamProc = divert.MustFindProc("WinDivertGetParam")
)

type LAYER int

const (
	LAYER_NETWORK         LAYER = iota // Network layer.
	LAYER_NETWORK_FORWARD              // Network layer (forwarded packets)
	LAYER_FLOW                         // Flow layer.
	LAYER_SOCKET                       // Socket layer.
	LAYER_REFLECT                      // Reflect layer.
)

type ADDRESS struct {
	Timestamp int64
	Header    struct {
		Layer uint8 // Packet's layer.
		Event Event // Packet event.

		// UINT32 Sniffed : 1;                   /* Packet was sniffed? */
		// UINT32 Outbound : 1;                  /* Packet is outound? */
		// UINT32 Loopback : 1;                  /* Packet is loopback? */
		// UINT32 Impostor : 1;                  /* Packet is impostor? */
		// UINT32 IPv6 : 1;                      /* Packet is IPv6? */
		// UINT32 IPChecksum : 1;                /* Packet has valid IPv4 checksum? */
		// UINT32 TCPChecksum : 1;               /* Packet has valid TCP checksum? */
		// UINT32 UDPChecksum : 1;               /* Packet has valid UDP checksum? */
		Flags uint8

		reserved1 uint8
	}
	Size uint32 //

	// DATA_NETWORK Network;   // Network layer data.
	// DATA_FLOW Flow;         // Flow layer data.
	// DATA_SOCKET Socket;     // Socket layer data.
	// DATA_REFLECT Reflect;   // Reflect layer data.
	reserved3 [64]byte
}

func (a *ADDRESS) Sniffed() bool {
	return a.Header.Flags&0x01 != 0
}

func (a *ADDRESS) Outbound() bool {
	return a.Header.Flags&0x02 != 0
}

func (a *ADDRESS) Loopback() bool {
	return a.Header.Flags&0x04 != 0
}

func (a *ADDRESS) Impostor() bool {
	return a.Header.Flags&0x08 != 0
}

func (a *ADDRESS) IPv6() bool {
	return a.Header.Flags&0x10 != 0
}

func (a *ADDRESS) IPChecksum() bool {
	return a.Header.Flags&0x20 != 0
}

func (a *ADDRESS) TCPChecksum() bool {
	return a.Header.Flags&0x40 != 0
}

func (a *ADDRESS) Network() *DATA_NETWORK {
	return (*DATA_NETWORK)(unsafe.Pointer(&a.reserved3[0]))
}
func (a *ADDRESS) Flow() *DATA_FLOW {
	return (*DATA_FLOW)(unsafe.Pointer(&a.reserved3[0]))
}
func (a *ADDRESS) Socket() *DATA_SOCKET {
	return (*DATA_SOCKET)(unsafe.Pointer(&a.reserved3[0]))
}
func (a *ADDRESS) Reflect() *DATA_REFLECT {
	return (*DATA_REFLECT)(unsafe.Pointer(&a.reserved3[0]))
}

type DATA_NETWORK struct {
	IfIdx    uint32 // Packet's interface index.
	SubIfIdx uint32 // Packet's sub-interface index.
}

type DATA_FLOW struct {
	EndpointId       uint64    // Endpoint ID.
	ParentEndpointId uint64    // Parent endpoint ID.
	ProcessId        uint32    // Process ID.
	localAddr        [4]uint32 // Local address.
	remoteAddr       [4]uint32 // Remote address.
	LocalPort        uint16    // Local port.
	RemotePort       uint16    // Remote port.
	Protocol         Proto     // Protocol.
}

func (d *DATA_FLOW) LocalAddr() netip.Addr {
	return netip.AddrFrom16(*(*[16]byte)(unsafe.Pointer(&d.localAddr)))
}

func (d *DATA_FLOW) RemoteAddr() netip.Addr {
	return netip.AddrFrom16(*(*[16]byte)(unsafe.Pointer(&d.remoteAddr)))
}

type DATA_SOCKET = DATA_FLOW

type DATA_REFLECT struct {
	Timestamp int64  // Handle open time.
	ProcessId uint32 // Handle process ID.
	Layer     LAYER  // Handle layer.
	Flags     uint64 // Handle flags.
	Priority  int16  // Handle priority.
}

type Handle windows.Handle

func Open(filter string, layer LAYER, priority int16, flags Flag) (Handle, error) {
	pf, err := windows.BytePtrFromString(filter)
	if err != nil {
		return Handle(0), err
	}

	r1, _, err := winDivertOpenProc.Call(uintptr(unsafe.Pointer(pf)), uintptr(layer), uintptr(priority), uintptr(flags))
	h := Handle(r1)
	if h == INVALID_HANDLE_VALUE {
		return Handle(0), err
	}
	return h, nil
}

func (h Handle) Recv(packet []byte) (int, ADDRESS, error) {
	var recvLen uint32
	var addr ADDRESS

	sp := (*reflect.SliceHeader)(unsafe.Pointer(&packet))
	r1, _, err := winDivertRecvProc.Call(
		uintptr(h),
		sp.Data,
		uintptr(sp.Len),
		uintptr(unsafe.Pointer(&recvLen)),
		uintptr(unsafe.Pointer(&addr)),
	)
	if r1 == 0 {
		return 0, addr, err
	}
	return int(recvLen), addr, nil
}

type OVERLAPPED windows.Overlapped
type LPOVERLAPPED *OVERLAPPED

func (h Handle) RecvEx(
	packet []byte, flag uint64,
	lpOverlapped LPOVERLAPPED,
) (int, ADDRESS, error) {

	var recvLen uint32
	var addr ADDRESS
	sp := (*reflect.SliceHeader)(unsafe.Pointer(&packet))
	r1, _, err := winDivertRecvExProc.Call(
		uintptr(h),
		sp.Data,
		uintptr(sp.Len),
		uintptr(unsafe.Pointer(&recvLen)),
		uintptr(flag),
		uintptr(unsafe.Pointer(&addr)),
		uintptr(unsafe.Pointer(&addr.Size)),
		uintptr(unsafe.Pointer(lpOverlapped)),
	)
	if r1 == 0 {
		return 0, addr, err
	}
	return int(recvLen), addr, nil
}

func (h Handle) Send(
	packet []byte,
	pAddr *ADDRESS,
) (int, error) {

	var pSendLen uint32
	sp := (*reflect.SliceHeader)(unsafe.Pointer(&packet))
	r1, _, err := winDivertSendProc.Call(
		uintptr(h),
		sp.Data,
		uintptr(sp.Len),
		uintptr(unsafe.Pointer(&pSendLen)),
		uintptr(unsafe.Pointer(pAddr)),
	)

	if r1 == 0 {
		return 0, err
	}
	return int(pSendLen), nil
}

func (h Handle) SendEx(
	packet []byte, flag uint64,
	pAddr *ADDRESS,
) (int, LPOVERLAPPED, error) {

	var pSendLen uint32
	var overlapped OVERLAPPED
	sp := (*reflect.SliceHeader)(unsafe.Pointer(&packet))

	r1, _, err := winDivertSendExProc.Call(
		uintptr(h),
		sp.Data,
		uintptr(sp.Len),
		uintptr(unsafe.Pointer(&pSendLen)),
		uintptr(flag),
		uintptr(unsafe.Pointer(pAddr)),
		uintptr(pAddr.Size),
		uintptr(unsafe.Pointer(&overlapped)),
	)

	if r1 == 0 {
		return 0, nil, err
	}
	return int(pSendLen), &overlapped, nil
}

type SHUTDOWN uint32

const (
	SHUTDOWN_RECV           SHUTDOWN = iota + 1 /* Shutdown recv. */
	SHUTDOWN_SEND                               /* Shutdown send. */
	WINDIVERT_SHUTDOWN_BOTH                     /* Shutdown recv and send. */
)

func (h Handle) Shutdown(how SHUTDOWN) error {
	r1, _, err := winDivertShutdownProc.Call(uintptr(h), uintptr(how))
	if r1 == 0 {
		return err
	}
	return nil
}

func (h Handle) Close() error {
	r1, _, err := winDivertCloseProc.Call(uintptr(h))
	if r1 == 0 {
		return err
	}
	return nil
}

type PARAM uint32

const (
	QUEUE_LENGTH  PARAM = iota /* Packet queue length. */
	QUEUE_TIME                 /* Packet queue time. */
	QUEUE_SIZE                 /* Packet queue size. */
	VERSION_MAJOR              /* Driver version (major). */
	VERSION_MINOR              /* Driver version (minor). */
)

func (h Handle) SetParam(param PARAM, value uint64) error {
	r1, _, err := winDivertSetParamProc.Call(uintptr(h), uintptr(param), uintptr(value))
	if r1 == 0 {
		return err
	}
	return nil
}

func (h Handle) GetParamProc(param PARAM) (value uint64, err error) {
	r1, _, err := winDivertGetParamProc.Call(uintptr(h), uintptr(param), uintptr(unsafe.Pointer(&value)))
	if r1 == 0 {
		return 0, err
	}
	return value, nil
}
