package xlswriter

import (
	"bytes"
	"math"
	"strings"
	"unicode/utf16"
)

const (
	olePpsTypeRoot   = 5
	olePpsTypeDir    = 1
	olePpsTypeFile   = 2
	oleDataSizeSmall = 0x1000
	oleLongIntSize   = 4
	olePpsSize       = 0x80
)

type dataSectionItem struct {
	summary    uint32
	offset     uint32
	sType      uint32
	dataInt    uint32
	dataString string
	dataLength uint32
}

func saveBbd(buffer *bytes.Buffer, iSbdSize, iBsize, iPpsCnt uint32, entries []pps) {
	// Calculate Basic Setting
	var iBbCnt uint32 = 512 / oleLongIntSize
	var i1stBdL uint32 = (512 - 0x4C) / oleLongIntSize

	var iBdExL uint32 = 0
	iAll := iBsize + iPpsCnt + iSbdSize
	iAllW := iAll
	iBdCntW := uint32(math.Floor(float64(iAllW) / float64(iBbCnt)))
	if iAllW%iBbCnt > 0 {
		iBdCntW++
	}
	iBdCnt := uint32(math.Floor(float64(iAll+iBdCntW) / float64(iBbCnt)))
	if (iAllW+iBdCntW)%iBbCnt > 0 {
		iBdCnt++
	}
	// Calculate BD count
	if iBdCnt > i1stBdL {
		for {
			iBdExL++
			iAllW++
			iBdCntW = uint32(math.Floor(float64(iAllW) / float64(iBbCnt)))
			if iAllW%iBbCnt > 0 {
				iBdCntW++
			}
			iBdCnt = uint32(math.Floor(float64(iAllW+iBdCntW) / float64(iBbCnt)))
			if (iAllW+iBdCntW)%iBbCnt > 0 {
				iBdCnt++
			}
			if iBdCnt <= (iBdExL*iBbCnt + i1stBdL) {
				break
			}
		}
	}

	// Making BD
	// Set for SBD
	if iSbdSize > 0 {
		var i uint32
		for i = 0; i < (iSbdSize - 1); i++ {
			putVar(buffer, i+1)
		}
		putVar(buffer, []byte("\xFE\xFF\xFF\xFF")) // uint32(-2)
	}

	// Each large stream has its own FAT chain (including the root mini stream).
	var i uint32
	block := iSbdSize
	for _, entry := range entries {
		if entry.Size < oleDataSizeSmall && entry.PpsType != olePpsTypeRoot {
			continue
		}
		count := (uint32(len(entry.Data)) + 511) / 512
		for j := uint32(0); j < count; j++ {
			if j+1 == count {
				putVar(buffer, uint32(0xFFFFFFFE))
			} else {
				putVar(buffer, block+1)
			}
			block++
		}
	}

	// Set for PPS
	for i = 0; i < (iPpsCnt - 1); i++ {
		putVar(buffer, i+iSbdSize+iBsize+1)
	}
	putVar(buffer, []byte("\xFE\xFF\xFF\xFF"))

	// Set for BBD itself ( 0xFFFFFFFD : BBD)
	for i = 0; i < iBdCnt; i++ {
		putVar(buffer, uint32(0xFFFFFFFD))
	}

	// Set for ExtraBDList
	for i = 0; i < iBdExL; i++ {
		putVar(buffer, uint32(0xFFFFFFFC))
	}

	// Adjust for Block
	if (iAllW+iBdCnt)%iBbCnt > 0 {
		iBlock := iBbCnt - ((iAllW + iBdCnt) % iBbCnt)
		for i = 0; i < iBlock; i++ {
			putVar(buffer, []byte("\xFF\xFF\xFF\xFF"))
		}
	}

	// Extra BDList
	if iBdCnt > i1stBdL {
		var iN, iNb uint32
		for i = i1stBdL; i < iBdCnt; i++ {
			if iN >= (iBbCnt - 1) {
				iN = 0
				iNb++
				putVar(buffer, iAll+iBdCnt+iNb)
			}
			putVar(buffer, iBsize+iSbdSize+iPpsCnt+i)
			iN++
		}
		if (iBdCnt-i1stBdL)%(iBbCnt-1) > 0 {
			iB := (iBbCnt - 1) - ((iBdCnt - i1stBdL) % (iBbCnt - 1))
			for i = 0; i < iB; i++ {
				putVar(buffer, []byte("\xFF\xFF\xFF\xFF"))
			}
		}
		putVar(buffer, []byte("\xFE\xFF\xFF\xFF"))
	}
}

func savePps(buffer *bytes.Buffer, raList []pps) {
	// Save each PPS WK
	for _, pps := range raList {
		putVar(buffer, []byte(pps.getPpsWk())) // maybe it'll be better to change return type to []byte
	}
	// Adjust for Block
	iCnt := len(raList)
	iBCnt := 512 / olePpsSize
	if iCnt%iBCnt > 0 {
		putVar(buffer, []byte(strings.Repeat("\x00", (iBCnt-(iCnt%iBCnt))*olePpsSize)))
	}
}

func saveBigData(buffer *bytes.Buffer, iStBlk uint32, raList []pps) {
	// cycle through PPS's
	for i, _ := range raList {
		if raList[i].PpsType != olePpsTypeDir {
			raList[i].Size = uint32(len(raList[i].Data))
			if raList[i].Size >= oleDataSizeSmall || (raList[i].PpsType == olePpsTypeRoot && len(raList[i].Data) != 0) {
				putVar(buffer, []byte(raList[i].Data))

				if raList[i].Size%512 > 0 {
					putVar(buffer, []byte(strings.Repeat("\x00", 512-int(raList[i].Size)%512)))
				}
				// Set For PPS
				raList[i].StartBlock = iStBlk
				iStBlk += uint32(math.Floor(float64(raList[i].Size) / 512))
				if raList[i].Size%512 > 0 {
					iStBlk++
				}
			}
		}
	}
}

func makeSmallData(buffer *bytes.Buffer, raList []pps) string {
	var smallData strings.Builder
	var iSmBlk uint32 = 0

	for i, _ := range raList {
		// Make SBD, small data string
		if raList[i].PpsType == olePpsTypeFile {
			if raList[i].Size <= 0 {
				continue
			}

			if raList[i].Size < oleDataSizeSmall {
				iSmbCnt := uint32(math.Floor(float64(raList[i].Size) / 64))
				if raList[i].Size%64 > 0 {
					iSmbCnt++
				}
				jB := iSmbCnt - 1
				var j uint32
				for j = 0; j < jB; j++ {
					putVar(buffer, j+iSmBlk+1)
				}
				putVar(buffer, []byte("\xFE\xFF\xFF\xFF")) // uint32(-2)

				smallData.WriteString(raList[i].Data)
				if raList[i].Size%64 > 0 {
					smallData.WriteString(strings.Repeat("\x00", 64-int(raList[i].Size%64)))
				}
				// Set for PPS
				raList[i].StartBlock = iSmBlk
				iSmBlk += iSmbCnt
			}
		}
	}

	iSbCnt := uint32(math.Floor(512.0 / oleLongIntSize))
	if iSmBlk%iSbCnt > 0 {
		iB := iSbCnt - (iSmBlk % iSbCnt)
		var i uint32
		for i = 0; i < iB; i++ {
			putVar(buffer, []byte("\xFF\xFF\xFF\xFF"))
		}
	}

	return smallData.String()
}

func saveHeader(buffer *bytes.Buffer, iSBDcnt, iBBcnt, iPPScnt uint32) {
	// Calculate Basic Setting
	var iBlCnt uint32 = 512 / oleLongIntSize
	var i1stBdL uint32 = (512 - 0x4C) / oleLongIntSize

	var iBdExL uint32 = 0
	iAll := uint32(iBBcnt + iPPScnt + iSBDcnt)
	iAllW := iAll
	iBdCntW := uint32(math.Floor(float64(iAllW) / float64(iBlCnt)))
	if iAllW%iBlCnt > 0 {
		iBdCntW++
	}
	iBdCnt := uint32(math.Floor(float64(iAll+iBdCntW) / float64(iBlCnt)))
	if (iAllW+iBdCntW)%iBlCnt > 0 {
		iBdCnt++
	}

	// Calculate BD count
	if iBdCnt > i1stBdL {
		for {
			iBdExL++
			iAllW++
			iBdCntW = uint32(math.Floor(float64(iAllW) / float64(iBlCnt)))
			if iAllW%iBlCnt > 0 {
				iBdCntW++
			}
			iBdCnt = uint32(math.Floor(float64(iAllW+iBdCntW) / float64(iBlCnt)))
			if (iAllW+iBdCntW)%iBlCnt > 0 {
				iBdCnt++
			}
			if iBdCnt <= (iBdExL*iBlCnt + i1stBdL) {
				break
			}
		}
	}

	// Save Header
	putVar(buffer,
		[]byte("\xD0\xCF\x11\xE0\xA1\xB1\x1A\xE1"),
		[]byte("\x00\x00\x00\x00"),
		[]byte("\x00\x00\x00\x00"),
		[]byte("\x00\x00\x00\x00"),
		[]byte("\x00\x00\x00\x00"),
		uint16(0x3b),
		uint16(0x03),
		[]byte("\xFE\xFF"), // uint16(-2),
		uint16(9),
		uint16(6),
		uint16(0),
		[]byte("\x00\x00\x00\x00"),
		[]byte("\x00\x00\x00\x00"),
		iBdCnt,
		iBBcnt+iSBDcnt,
		uint32(0),
		uint32(0x1000),
	)
	if iSBDcnt > 0 {
		putVar(buffer, uint32(0))
	} else {
		putVar(buffer, []byte("\xFE\xFF\xFF\xFF"))
	}
	putVar(buffer, iSBDcnt)

	// Extra BDList Start, Count
	if iBdCnt <= i1stBdL {
		putVar(buffer,
			[]byte("\xFE\xFF\xFF\xFF"), // Extra BDList Start
			uint32(0),                  // Extra BDList Count
		)
	} else {
		putVar(buffer, iAll+iBdCnt, iBdExL)
	}

	// BDList
	var i uint32
	for i = 0; i < i1stBdL && i < iBdCnt; i++ {
		putVar(buffer, iAll+i)
	}
	if i < i1stBdL {
		jB := i1stBdL - i
		var j uint32
		for j = 0; j < jB; j++ {
			putVar(buffer, []byte("\xFF\xFF\xFF\xFF"))
		}
	}
}

func calcSize(aList []pps) (uint32, uint32, uint32) {
	var iSBDcnt, iBBcnt, iPPScnt uint32 = 0, 0, 0

	iSBcnt := 0
	iCount := len(aList)
	for i := 0; i < iCount; i++ {
		if aList[i].PpsType == olePpsTypeFile {
			aList[i].Size = uint32(len(aList[i].Data))

			if aList[i].Size < oleDataSizeSmall {
				iSBcnt += int(math.Floor(float64(aList[i].Size) / 64))
				if aList[i].Size%64 > 0 {
					iSBcnt++
				}
			} else {
				iBBcnt += uint32(math.Floor(float64(aList[i].Size) / 512))
				if aList[i].Size%512 > 0 {
					iBBcnt++
				}
			}
		}
	}

	iSlCnt := int(math.Floor(512 / oleLongIntSize))
	iSBDcnt = uint32((iSBcnt + iSlCnt - 1) / iSlCnt)

	iSmallLen := float64(iSBcnt) * 64
	iBBcnt += uint32(math.Floor(iSmallLen / 512))
	if int(iSmallLen)%512 > 0 {
		iBBcnt++
	}
	iCnt := len(aList)
	iBdCnt := float64(512) / olePpsSize
	iPPScnt = uint32(math.Floor(float64(iCnt) / iBdCnt))
	if iCnt%int(iBdCnt) > 0 {
		iPPScnt++
	}

	return iSBDcnt, iBBcnt, iPPScnt
}

func getSummaryInformation(title, subject, creator, keywords, description, lastModifiedBy string, created, modified int64) string {
	buffer := new(bytes.Buffer)

	// offset: 0; size: 2; must be 0xFE 0xFF (UTF-16 LE byte order mark)
	putVar(buffer, uint16(0xFFFE))
	// offset: 2; size: 2;
	putVar(buffer, uint16(0x0000))
	// offset: 4; size: 2; OS version
	putVar(buffer, uint16(0x0106))
	// offset: 6; size: 2; OS indicator
	putVar(buffer, uint16(0x0002))
	// offset: 8; size: 16
	putVar(buffer, uint32(0x00), uint32(0x00), uint32(0x00), uint32(0x00))
	// offset: 24; size: 4; section count
	putVar(buffer, uint32(0x0001))

	// offset: 28; size: 16; first section's class id: 02 d5 cd d5 9c 2e 1b 10 93 97 08 00 2b 2c f9 ae
	putVar(buffer, uint16(0x85E0), uint16(0xF29F), uint16(0x4FF9), uint16(0x1068), uint16(0x91AB), uint16(0x0008), uint16(0x272B), uint16(0xD9B3))
	// offset: 44; size: 4; offset of the start
	putVar(buffer, uint32(0x30))

	var dataSectionNumProps uint32 = 0
	dataSections := make([]dataSectionItem, 0)

	// CodePage : CP-1252
	dataSections = append(dataSections, dataSectionItem{0x01, 0, 0x02, 1200, "", 0})
	dataSectionNumProps++

	// Title
	if title != "" {
		dataSections = append(dataSections, dataSectionItem{0x02, 0, 0x1F, 0, title, uint32(len(title))})
		dataSectionNumProps++
	}

	// Subject
	if subject != "" {
		dataSections = append(dataSections, dataSectionItem{0x03, 0, 0x1F, 0, subject, uint32(len(subject))})
		dataSectionNumProps++
	}

	// Author (Creator)
	if creator != "" {
		dataSections = append(dataSections, dataSectionItem{0x04, 0, 0x1F, 0, creator, uint32(len(creator))})
		dataSectionNumProps++
	}

	// Keywords
	if keywords != "" {
		dataSections = append(dataSections, dataSectionItem{0x05, 0, 0x1F, 0, keywords, uint32(len(keywords))})
		dataSectionNumProps++
	}

	// Comments (Description)
	if description != "" {
		dataSections = append(dataSections, dataSectionItem{0x06, 0, 0x1F, 0, description, uint32(len(description))})
		dataSectionNumProps++
	}

	// Last Saved By (LastModifiedBy)
	if lastModifiedBy != "" {
		dataSections = append(dataSections, dataSectionItem{0x08, 0, 0x1F, 0, lastModifiedBy, uint32(len(lastModifiedBy))})
		dataSectionNumProps++
	}

	// Created Date/Time
	if created != 0 {
		dataSections = append(dataSections, dataSectionItem{0x0C, 0, 0x40, 0, localDateToOLE(created), 0})
		dataSectionNumProps++
	}

	// Modified Date/Time
	if modified != 0 {
		dataSections = append(dataSections, dataSectionItem{0x0D, 0, 0x40, 0, localDateToOLE(modified), 0})
		dataSectionNumProps++
	}

	// Security
	dataSections = append(dataSections, dataSectionItem{0x13, 0, 0x03, 0x00, "", 0})
	dataSectionNumProps++

	dataSectionSummary := new(bytes.Buffer)
	dataSectionContent := new(bytes.Buffer)
	dataSectionContentOffset := 8 + dataSectionNumProps*8

	for _, dataSection := range dataSections {
		// Summary
		putVar(dataSectionSummary, dataSection.summary)
		// Offset
		putVar(dataSectionSummary, dataSectionContentOffset)
		// DataType
		putVar(dataSectionContent, dataSection.sType)
		// Data
		if dataSection.sType == 0x02 { // 2 byte signed integer
			putVar(dataSectionContent, dataSection.dataInt)
			dataSectionContentOffset += 8
		} else if dataSection.sType == 0x03 { // 4 byte signed integer
			putVar(dataSectionContent, dataSection.dataInt)
			dataSectionContentOffset += 8
		} else if dataSection.sType == 0x1F { // null-terminated string prepended by dword string length
			units := utf16.Encode([]rune(dataSection.dataString + "\x00"))
			putVar(dataSectionContent, uint32(len(units)), units)
			padding := (4 - len(units)*2%4) % 4
			putVar(dataSectionContent, make([]byte, padding))
			dataSectionContentOffset += uint32(8 + len(units)*2 + padding)
		} else if dataSection.sType == 0x40 { // Filetime (64-bit value representing the number of 100-nanosecond intervals since January 1, 1601)
			putVar(dataSectionContent, []byte(dataSection.dataString))
			dataSectionContentOffset += 4 + 8
		}
		// Data Type Not Used at the moment
	}
	// Now dataSectionContentOffset contains the size of the content

	// section header
	// offset: $secOffset; size: 4; section length
	//         + x  Size of the content (summary + content)
	putVar(buffer, dataSectionContentOffset)

	// offset: $secOffset+4; size: 4; property count
	putVar(buffer, dataSectionNumProps)

	// Section Summary
	putVar(buffer, dataSectionSummary.Bytes())

	// Section Content
	putVar(buffer, dataSectionContent.Bytes())

	return buffer.String()
}

// stringCollection ...
type stringCollection struct {
	stringMap    map[string]int
	stringList   []string
	stringTotal  int
	stringUnique int
}

func (sc *stringCollection) addRow(row []string) {

	for _, str := range row {
		if str == "" {
			continue
		}
		strToSave := utf8toBIFF8UnicodeLong(str)
		if _, ok := sc.stringMap[strToSave]; !ok {
			sc.stringMap[strToSave] = sc.stringUnique
			sc.stringList = append(sc.stringList, strToSave)
			sc.stringUnique++
		}

		sc.stringTotal++
	}
}
