package diskstats

import (
	"bufio"
	"errors"
	"io"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	deviceNameCol = 2
	readsCol      = 3
	writesCol     = 7
)

type DiskStats struct {
	Name        string
	IdleTime    int
	CommandType string
	Reads       int
	Writes      int
	SpinDownAt  time.Time
	SpinUpAt    time.Time
	LastIoAt    time.Time
	SpunDown    bool
}

var scsiDiskRegex *regexp.Regexp

func init() {
	scsiDiskRegex = regexp.MustCompile("(sd[a-z])[1-9]$")
}

func Snapshot() []DiskStats {
	f, err := os.Open("/proc/diskstats")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	return ReadSnapshot(f)
}

func ReadSnapshot(r io.Reader) []DiskStats {
	var snapshot []DiskStats
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		diskStats, err := statsForDisk(scanner.Text())
		if err == nil {
			len := len(snapshot)
			if len > 0 {
				prev := snapshot[len-1]
				if prev.Name == diskStats.Name {
					snapshot[len-1].Reads += diskStats.Reads
					snapshot[len-1].Writes += diskStats.Writes
					continue
				}
			}

			snapshot = append(snapshot, *diskStats)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	return snapshot
}

func statsForDisk(rawStats string) (*DiskStats, error) {
	reader := strings.NewReader(rawStats)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		cols := strings.Fields(scanner.Text())
		name := cols[deviceNameCol]
		reads, _ := strconv.Atoi(cols[readsCol])
		writes, _ := strconv.Atoi(cols[writesCol])

		part := scsiDiskRegex.FindStringSubmatch(name)
		if part == nil {
			return nil, errors.New("disk is not a partition")
		}
		stats := &DiskStats{
			Name:   part[1],
			Reads:  reads,
			Writes: writes,
		}
		return stats, nil
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return nil, errors.New("cannot read disk stats")
}
