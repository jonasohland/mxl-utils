package mxl

import (
	"os"
	"syscall"
)

func getInoFromStat(s os.FileInfo) (uint64, error) {
	sysInfo := s.Sys()
	if sysInfo == nil {
		return 0, os.ErrInvalid
	}

	uSysInfo, ok := sysInfo.(*syscall.Stat_t)
	if !ok {
		return 0, os.ErrInvalid
	}

	return uSysInfo.Ino, nil
}

func getIno(filename string) (uint64, error) {
	stat, err := os.Stat(filename)
	if err != nil {
		return 0, err
	}

	return getInoFromStat(stat)
}

func fgetIno(fd *os.File) (uint64, error) {
	stat, err := fd.Stat()
	if err != nil {
		return 0, err
	}

	return getInoFromStat(stat)
}
