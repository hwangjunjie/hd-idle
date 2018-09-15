package hdidle

import (
	"fmt"
	"github.com/adelolmo/hd-idle/diskstats"
	"github.com/adelolmo/hd-idle/sgio"
	"log"
	"os"
	"time"
)

const (
	SCSI        = "scsi"
	ATA         = "ata"
	DATE_FORMAT = "2006-01-02T15:04:05"
)

type DefaultConf struct {
	Idle        int
	CommandType string
	Debug       bool
	LogFile     string
}

type DeviceConf struct {
	Name        string
	Idle        int
	CommandType string
}

type Config struct {
	Devices  []DeviceConf
	Defaults DefaultConf
}

var previousSnapshots []diskstats.DiskStats

func ObserveDiskActivity(config *Config) {
	actualSnapshot := diskstats.TakeSnapshot()

	for _, stats := range actualSnapshot {
		updateState(stats, config)
	}
}

func updateState(tmp diskstats.DiskStats, config *Config) {
	dsi := previousDiskStatsIndex(tmp.Name)
	if dsi < 0 {
		previousSnapshots = append(previousSnapshots, initDevice(tmp, config))
		return
	}

	ds := previousSnapshots[dsi]
	if ds.Writes == tmp.Writes && ds.Reads == tmp.Reads {
		if !ds.SpunDown {
			/* no activity on this disk and still running */
			idleDuration := int(time.Now().Sub(ds.LastIoAt).Seconds())
			if ds.IdleTime != 0 && idleDuration > ds.IdleTime {
				spindownDisk(ds.Name, ds.CommandType)
				previousSnapshots[dsi].SpinDownAt = time.Now()
				previousSnapshots[dsi].SpunDown = true
			}
		}

	} else {
		/* disk had some activity */
		if ds.SpunDown {
			/* disk was spun down, thus it has just spun up */
			if len(config.Defaults.LogFile) > 0 {
				logSpinup(ds, config.Defaults.LogFile)
			}
			previousSnapshots[dsi].SpinUpAt = time.Now()
		}
		previousSnapshots[dsi].Reads = tmp.Reads
		previousSnapshots[dsi].Writes = tmp.Writes
		previousSnapshots[dsi].LastIoAt = time.Now()
		previousSnapshots[dsi].SpunDown = false
	}

	ds = previousSnapshots[dsi]
	idleDuration := int(time.Now().Sub(ds.LastIoAt).Seconds())
	if config.Defaults.Debug {
		fmt.Printf("disk=%s command=%s spunDown=%t "+
			"reads=%d writes=%d idleTime=%d idleDuration=%d "+
			"spindown=%s spinup=%s lastIO=%s\n",
			ds.Name, ds.CommandType, ds.SpunDown,
			ds.Reads, ds.Writes, ds.IdleTime, idleDuration,
			ds.SpinDownAt.Format(DATE_FORMAT), ds.SpinUpAt.Format(DATE_FORMAT), ds.LastIoAt.Format(DATE_FORMAT))
	}
}

func previousDiskStatsIndex(diskName string) int {
	for i, stats := range previousSnapshots {
		if stats.Name == diskName {
			return i
		}
	}
	return -1
}

func initDevice(stats diskstats.DiskStats, config *Config) diskstats.DiskStats {
	idle := config.Defaults.Idle
	command := config.Defaults.CommandType
	deviceConf := deviceConfig(config, stats.Name)
	if deviceConf != nil {
		idle = deviceConf.Idle
		command = deviceConf.CommandType
	}

	return diskstats.DiskStats{
		Name:        stats.Name,
		LastIoAt:    time.Now(),
		SpinUpAt:    time.Now(),
		SpunDown:    false,
		Writes:      stats.Writes,
		Reads:       stats.Reads,
		IdleTime:    idle,
		CommandType: command,
	}
}

func deviceConfig(config *Config, diskName string) *DeviceConf {
	for _, device := range config.Devices {
		if device.Name == diskName {
			return &device
		}
	}
	return &DeviceConf{
		Name:        diskName,
		CommandType: config.Defaults.CommandType,
		Idle:        config.Defaults.Idle,
	}
}

func spindownDisk(deviceName, command string) {
	fmt.Printf("%s spindown\n", deviceName)
	device := fmt.Sprintf("/dev/%s", deviceName)
	switch command {
	case SCSI:
		sgio.StopScsiDevice(device)
		return
	case ATA:
		sgio.StopAtaDevice(device)
		return
	}
}

func logSpinup(ds diskstats.DiskStats, file string) {
	cacheFile, err := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		log.Fatalf("Cannot open file %s. Error: %s", file, err)
	}
	defer cacheFile.Close()
	now := time.Now()
	text := fmt.Sprintf("date: %s, time: %s, disk: %s, running: %d, stopped: %d\n",
		now.Format("2006-01-02"), now.Format("15:04:05"), ds.Name,
		int(ds.SpinDownAt.Sub(ds.SpinUpAt).Seconds()), int(now.Sub(ds.SpinDownAt).Seconds()))
	if _, err = cacheFile.WriteString(text); err != nil {
		log.Fatalf("Cannot write into file %s. Error: %s", file, err)
	}
}

func (c *Config) String() string {
	return fmt.Sprintf("defaultIdle=%d, defaultCommand=%s, debug=%t devices=%v",
		c.Defaults.Idle, c.Defaults.CommandType, c.Defaults.Debug, c.Devices)
}