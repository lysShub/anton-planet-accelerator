package event

import (
	"sync"
	"unsafe"

	_ "unsafe"

	"golang.org/x/sys/windows"
)

type processListener struct {
	mu             sync.RWMutex
	currWalkStatue bool

	nameMap map[string]struct{}
	pidMap  map[uint32]*processElem
}

type ProcessEvent struct {
	Name  string
	Pid   uint32
	start bool
}

func (r *ProcessEvent) Start() bool { return r.start }

func NewProcessListen() *processListener {
	var p = &processListener{
		nameMap: map[string]struct{}{},
		pidMap:  map[uint32]*processElem{},
	}
	return p
}

type processElem struct {
	name       string
	walkStatue bool
}

func (p *processListener) Register(proc string) *processListener {
	p.mu.Lock()
	p.nameMap[proc] = struct{}{}
	p.mu.Unlock()

	return p
}

func (p *processListener) Delete(name string) *processListener {
	p.mu.Lock()
	delete(p.nameMap, name)
	p.mu.Unlock()

	return p
}

func (p *processListener) Walk() ([]ProcessEvent, error) {
	var pes []ProcessEvent
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.nameMap) == 0 {
		return pes, nil
	}

	const syspid = 4
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, syspid)
	if err != nil {
		return pes, err
	}
	defer windows.CloseHandle(snap)
	var pe32 windows.ProcessEntry32
	pe32.Size = uint32(unsafe.Sizeof(pe32))
	if err = windows.Process32First(snap, &pe32); err != nil {
		return pes, err
	}

	p.currWalkStatue = !p.currWalkStatue
	for {
		if pid := pe32.ProcessID; pid > syspid {
			name := windows.UTF16ToString(pe32.ExeFile[:])
			if _, has := p.nameMap[name]; has {
				if e, has := p.pidMap[pid]; has {
					e.walkStatue = p.currWalkStatue
				} else {
					p.pidMap[pid] = &processElem{
						name:       name,
						walkStatue: p.currWalkStatue,
					}
					pes = append(pes, ProcessEvent{Name: name, start: true, Pid: pid})
				}
			}
		}

		if err = windows.Process32Next(snap, &pe32); err != nil {
			if err == windows.ERROR_NO_MORE_FILES {
				break
			} else {
				return nil, err
			}
		}
	}

	for pid, e := range p.pidMap {
		if e.walkStatue != p.currWalkStatue {
			_, has := p.nameMap[e.name]
			if has {
				pes = append(pes, ProcessEvent{Name: e.name, start: false, Pid: pid})
			}
			delete(p.pidMap, pid)
		}
	}
	return pes, nil
}
