//go:build windows

// I really don't have a good idea of the Windows APIs for stuff like this,
// so I took alot of things from wireguard-go's wintun/windows tunnel implementation
// https://github.com/WireGuard/wireguard-go/blob/master/tun/tun_windows.go
// which has the license
///* SPDX-License-Identifier: MIT
// *
// * Copyright (C) 2017-2023 WireGuard LLC. All Rights Reserved.
// */

package tun

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"sync"
	"sync/atomic"

	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wintun"
	"golang.zx2c4.com/wireguard/windows/tunnel/winipcfg"
)

var (
	WintunAdapaterName        = "WintunTestAdp"
	WuntunAdapaterType        = "Wintun"
	WintunStaticRequestedGUID *windows.GUID
)

type WinTun struct {
	name      string
	handle    windows.Handle // this is set to InvalidHandle?
	readWait  windows.Handle
	session   wintun.Session
	wt        *wintun.Adapter
	close     atomic.Bool
	closeOnce sync.Once
	running   sync.WaitGroup
	writeLock sync.Mutex // Currently used because I am calling write from multiple goroutines
}

func NewTun() (Tun, error) {
	wt, err := wintun.CreateAdapter(WintunAdapaterName, WuntunAdapaterType, WintunStaticRequestedGUID)
	if err != nil {
		return nil, fmt.Errorf("error creating wintun adapater: %w", err)
	}

	sess, err := wt.StartSession(0x800000) // 8Mib Ring Capacity
	if err != nil {
		wt.Close()
		return nil, errors.New("error starting wintun session")
	}

	rw := sess.ReadWaitEvent()

	return &WinTun{
		wt:       wt,
		name:     WintunAdapaterName,
		handle:   windows.InvalidHandle,
		readWait: rw,
		session:  sess,
	}, nil
}

func (tun *WinTun) Name() string {
	return WintunAdapaterName
}

func (tun *WinTun) Read(b []byte) (int, error) {
	tun.running.Add(1)
	defer tun.running.Done()

	if tun.close.Load() {
		return 0, os.ErrClosed
	}

retry:
	if tun.close.Load() {
		return 0, os.ErrClosed
	}
	packet, err := tun.session.ReceivePacket()

	switch err {
	case nil:
		n := copy(b, packet)
		tun.session.ReleaseReceivePacket(packet)
		return n, nil
	case windows.ERROR_NO_MORE_ITEMS:
		windows.WaitForSingleObject(tun.readWait, windows.INFINITE)
		goto retry
	case windows.ERROR_HANDLE_EOF:
		return 0, os.ErrClosed
	case windows.ERROR_INVALID_DATA:

		return 0, errors.New("Invalid Data")
	}
	return 0, fmt.Errorf("Read failed: %w", err)

}

func (tun *WinTun) Write(b []byte) (int, error) {
	tun.running.Add(1)
	defer tun.running.Done()
	if tun.close.Load() {
		return 0, os.ErrClosed
	}
	// Lock here since multiple routines are writing to tunnel at the moment
	tun.writeLock.Lock()
	defer tun.writeLock.Unlock()

	packet, err := tun.session.AllocateSendPacket(len(b))
	switch err {
	case nil:
		copy(packet, b)
		tun.session.SendPacket(packet)
		return len(b), nil
	case windows.ERROR_HANDLE_EOF:
		return 0, os.ErrClosed
	case windows.ERROR_BUFFER_OVERFLOW:
		return 0, errors.New("wintun send buffer full")
	default:
		return 0, fmt.Errorf("error writing to tunnel: %w", err)
	}

}

func (tun *WinTun) Close() error {
	var closeErr error
	tun.closeOnce.Do(func() {
		tun.close.Store(true)
		windows.SetEvent(tun.readWait)
		tun.running.Wait()
		tun.session.End()
		if tun.wt != nil {
			closeErr = tun.wt.Close()
		}
	})

	return closeErr
}

func (tun *WinTun) MTU() (int, error) {
	return 1400, nil
}

func (tun *WinTun) LUID() uint64 {
	tun.running.Add(1)
	defer tun.running.Done()
	if tun.close.Load() {
		return 0
	}
	return tun.wt.LUID()
}

func (tun *WinTun) ConfigureIPAddress(addr netip.Prefix) error {
	luid := winipcfg.LUID(tun.LUID())

	err := luid.AddIPAddress(addr)
	if err != nil {
		return err
	}
	return nil
}
