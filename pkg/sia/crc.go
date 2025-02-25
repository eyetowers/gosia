package sia

import (
	"github.com/getrak/crc16"
)

var table = crc16.MakeTable(crc16.CRC16_ARC)

func checksum(data []byte) uint16 {
	return crc16.Checksum(data, table)
}
