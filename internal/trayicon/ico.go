package trayicon

import "encoding/binary"

// PNGToICO wraps a square PNG in a single-image ICO. Windows trays reject raw PNG.
func PNGToICO(png []byte, size int) []byte {
	if size > 255 {
		size = 0
	}
	out := make([]byte, 22+len(png))
	binary.LittleEndian.PutUint16(out[0:], 0)
	binary.LittleEndian.PutUint16(out[2:], 1)
	binary.LittleEndian.PutUint16(out[4:], 1)
	out[6] = byte(size)
	out[7] = byte(size)
	out[8] = 0
	out[9] = 0
	binary.LittleEndian.PutUint16(out[10:], 1)
	binary.LittleEndian.PutUint16(out[12:], 32)
	binary.LittleEndian.PutUint32(out[14:], uint32(len(png)))
	binary.LittleEndian.PutUint32(out[18:], 22)
	copy(out[22:], png)
	return out
}
