package server_common

import (
	"context"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/offchainlabs/nitro/util/containers"
)

type MachineStatus[M any] struct {
	containers.Promise[*M]
}

func newMachineStatus[M any]() *MachineStatus[M] {
	return &MachineStatus[M]{
		Promise: containers.NewPromise[*M](),
	}
}

type MachineLoader[M any] struct {
	mapMutex            sync.Mutex
	machines            map[common.Hash]*MachineStatus[M]
	locator             *MachineLocator
	createMachineThread func(ctx context.Context, moduleRoot common.Hash) (*M, error)
}

func NewMachineLoader[M any](locator *MachineLocator,
	createMachineThread func(ctx context.Context, moduleRoot common.Hash) (*M, error)) *MachineLoader[M] {

	return &MachineLoader[M]{
		machines:            make(map[common.Hash]*MachineStatus[M]),
		locator:             locator,
		createMachineThread: createMachineThread,
	}
}

func (l *MachineLoader[M]) GetMachine(ctx context.Context, moduleRoot common.Hash) (*M, error) {
	if moduleRoot == (common.Hash{}) {
		moduleRoot = l.locator.LatestWasmModuleRoot()
		if (moduleRoot == common.Hash{}) {
			return nil, ErrMachineNotFound
		}
	}
	l.mapMutex.Lock()
	status := l.machines[moduleRoot]
	if status == nil {
		status = newMachineStatus[M]()
		l.machines[moduleRoot] = status
		go func() {
			machine, err := l.createMachineThread(context.Background(), moduleRoot)
			if err != nil {
				status.ProduceError(err)
				return
			}
			status.Produce(machine)
		}()
	}
	l.mapMutex.Unlock()
	return status.Await(ctx)
}

func (l *MachineLoader[M]) ForEachReadyMachine(runme func(*M) error) error {
	for _, stat := range l.machines {
		if stat.Ready() {
			machine, err := stat.Current()
			if err != nil {
				if err := runme(machine); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
