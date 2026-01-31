//go:build windows

package tray

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/lxn/walk"
	"github.com/lxn/win"
)

// 临时文件计数器，用于生成唯一文件名
var tempFileCounter uint64

// createIconFromICO 从 ICO 数据创建图标
func createIconFromICO(data []byte) (*walk.Icon, error) {
	if len(data) < 6 {
		return createFallbackIcon()
	}

	// 获取系统托盘图标尺寸
	cxIcon := int(win.GetSystemMetrics(win.SM_CXSMICON))
	cyIcon := int(win.GetSystemMetrics(win.SM_CYSMICON))

	// 使用唯一的临时文件名避免竞争
	counter := atomic.AddUint64(&tempFileCounter, 1)
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("claude-status-icon-%d.ico", counter))

	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return createFallbackIcon()
	}

	// 使用 LoadImage API 加载指定尺寸的图标
	user32 := syscall.NewLazyDLL("user32.dll")
	loadImage := user32.NewProc("LoadImageW")

	pathPtr, _ := syscall.UTF16PtrFromString(tmpFile)
	ret, _, _ := loadImage.Call(
		0, // hInstance
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(1), // IMAGE_ICON
		uintptr(cxIcon),
		uintptr(cyIcon),
		uintptr(0x00000010), // LR_LOADFROMFILE
	)

	// 加载完成后立即删除临时文件
	os.Remove(tmpFile)

	if ret == 0 {
		return createFallbackIcon()
	}

	hIcon := win.HICON(ret)
	icon, err := walk.NewIconFromHICONForDPI(hIcon, 96)
	if err != nil {
		win.DestroyIcon(hIcon)
		return createFallbackIcon()
	}
	return icon, nil
}


// createFallbackIcon 创建后备图标
func createFallbackIcon() (*walk.Icon, error) {
	img := createFallbackImage()
	return walk.NewIconFromImage(img)
}

// bytesToImage 将 ICO 字节数据转换为 image.Image
// ICO 文件格式：https://en.wikipedia.org/wiki/ICO_(file_format)
func bytesToImage(data []byte) image.Image {
	if len(data) < 22 {
		return createFallbackImage()
	}

	reader := bytes.NewReader(data)

	// ICO 头部
	var reserved uint16
	var imageType uint16
	var imageCount uint16

	binary.Read(reader, binary.LittleEndian, &reserved)
	binary.Read(reader, binary.LittleEndian, &imageType)
	binary.Read(reader, binary.LittleEndian, &imageCount)

	if reserved != 0 || imageType != 1 || imageCount == 0 {
		return createFallbackImage()
	}

	// 读取第一个图像目录条目（选择最大的图标）
	var bestEntry struct {
		width      uint8
		height     uint8
		colorCount uint8
		reserved   uint8
		planes     uint16
		bitCount   uint16
		size       uint32
		offset     uint32
	}
	var bestSize uint32 = 0

	var bestDimension int = 0

	for i := uint16(0); i < imageCount; i++ {
		var entry struct {
			width      uint8
			height     uint8
			colorCount uint8
			reserved   uint8
			planes     uint16
			bitCount   uint16
			size       uint32
			offset     uint32
		}

		binary.Read(reader, binary.LittleEndian, &entry.width)
		binary.Read(reader, binary.LittleEndian, &entry.height)
		binary.Read(reader, binary.LittleEndian, &entry.colorCount)
		binary.Read(reader, binary.LittleEndian, &entry.reserved)
		binary.Read(reader, binary.LittleEndian, &entry.planes)
		binary.Read(reader, binary.LittleEndian, &entry.bitCount)
		binary.Read(reader, binary.LittleEndian, &entry.size)
		binary.Read(reader, binary.LittleEndian, &entry.offset)

		// 获取实际尺寸（0 表示 256）
		w := int(entry.width)
		h := int(entry.height)
		if w == 0 {
			w = 256
		}
		if h == 0 {
			h = 256
		}

		// 选择最大的图标以获得最佳清晰度
		dimension := w * h
		if dimension > bestDimension {
			bestEntry = entry
			bestSize = entry.size
			bestDimension = dimension
		}
	}

	if bestSize == 0 || int(bestEntry.offset)+int(bestSize) > len(data) {
		return createFallbackImage()
	}

	// 获取图像数据
	imageData := data[bestEntry.offset : bestEntry.offset+bestSize]

	// 检查是否是 PNG（嵌入式 PNG）
	if len(imageData) > 8 && imageData[0] == 0x89 && imageData[1] == 0x50 &&
		imageData[2] == 0x4E && imageData[3] == 0x47 {
		// PNG 格式，使用 image/png 解码
		return decodePNG(imageData)
	}

	// BMP 格式（DIB）
	return decodeBMPFromICO(imageData, int(bestEntry.width), int(bestEntry.height), int(bestEntry.bitCount))
}

// decodePNG 解码 PNG 数据
func decodePNG(data []byte) image.Image {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return createFallbackImage()
	}
	return img
}

// decodeBMPFromICO 从 ICO 中的 BMP 数据解码图像
func decodeBMPFromICO(data []byte, width, height, bitCount int) image.Image {
	if width == 0 {
		width = 256
	}
	if height == 0 {
		height = 256
	}

	// ICO 中的 BMP 高度是实际高度的两倍（包含掩码）
	// DIB 头部
	if len(data) < 40 {
		return createFallbackImage()
	}

	// 跳过 BITMAPINFOHEADER（40 字节）
	headerSize := binary.LittleEndian.Uint32(data[0:4])
	if headerSize < 40 {
		return createFallbackImage()
	}

	bmpWidth := int(int32(binary.LittleEndian.Uint32(data[4:8])))
	bmpHeight := int(int32(binary.LittleEndian.Uint32(data[8:12])))
	bmpBitCount := int(binary.LittleEndian.Uint16(data[14:16]))

	if bmpWidth == 0 {
		bmpWidth = width
	}
	// ICO 中的高度是双倍的（图像+掩码）
	actualHeight := bmpHeight / 2
	if actualHeight == 0 {
		actualHeight = height
	}

	img := image.NewRGBA(image.Rect(0, 0, bmpWidth, actualHeight))

	// 根据位深度解析像素数据
	pixelDataOffset := int(headerSize)

	// 处理调色板（如果有）
	var palette []color.RGBA
	if bmpBitCount <= 8 {
		paletteSize := 1 << bmpBitCount
		palette = make([]color.RGBA, paletteSize)
		for i := 0; i < paletteSize && pixelDataOffset+i*4+4 <= len(data); i++ {
			idx := pixelDataOffset + i*4
			palette[i] = color.RGBA{
				B: data[idx],
				G: data[idx+1],
				R: data[idx+2],
				A: 255,
			}
		}
		pixelDataOffset += paletteSize * 4
	}

	// 计算每行字节数（4 字节对齐）
	rowSize := ((bmpWidth*bmpBitCount + 31) / 32) * 4

	// 读取像素数据（BMP 是自底向上的）
	for y := actualHeight - 1; y >= 0; y-- {
		rowOffset := pixelDataOffset + (actualHeight-1-y)*rowSize
		if rowOffset+rowSize > len(data) {
			continue
		}

		for x := 0; x < bmpWidth; x++ {
			var c color.RGBA

			switch bmpBitCount {
			case 32:
				idx := rowOffset + x*4
				if idx+4 <= len(data) {
					c = color.RGBA{
						B: data[idx],
						G: data[idx+1],
						R: data[idx+2],
						A: data[idx+3],
					}
				}
			case 24:
				idx := rowOffset + x*3
				if idx+3 <= len(data) {
					c = color.RGBA{
						B: data[idx],
						G: data[idx+1],
						R: data[idx+2],
						A: 255,
					}
				}
			case 8:
				idx := rowOffset + x
				if idx < len(data) && int(data[idx]) < len(palette) {
					c = palette[data[idx]]
				}
			case 4:
				idx := rowOffset + x/2
				if idx < len(data) {
					nibble := (data[idx] >> (4 * uint(1-x%2))) & 0x0F
					if int(nibble) < len(palette) {
						c = palette[nibble]
					}
				}
			case 1:
				idx := rowOffset + x/8
				if idx < len(data) {
					bit := (data[idx] >> (7 - uint(x%8))) & 0x01
					if int(bit) < len(palette) {
						c = palette[bit]
					}
				}
			}

			img.SetRGBA(x, y, c)
		}
	}

	// 处理 AND 掩码（透明度）
	if bmpBitCount < 32 {
		maskRowSize := ((bmpWidth + 31) / 32) * 4
		maskOffset := pixelDataOffset + rowSize*actualHeight

		for y := actualHeight - 1; y >= 0; y-- {
			maskRowOffset := maskOffset + (actualHeight-1-y)*maskRowSize
			if maskRowOffset+maskRowSize > len(data) {
				continue
			}

			for x := 0; x < bmpWidth; x++ {
				byteIdx := maskRowOffset + x/8
				if byteIdx < len(data) {
					bit := (data[byteIdx] >> (7 - uint(x%8))) & 0x01
					if bit == 1 {
						// 透明像素
						c := img.RGBAAt(x, y)
						c.A = 0
						img.SetRGBA(x, y, c)
					}
				}
			}
		}
	}

	return img
}

// createFallbackImage 创建后备图像
func createFallbackImage() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	// 创建一个简单的灰色方块
	gray := color.RGBA{128, 128, 128, 255}
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.SetRGBA(x, y, gray)
		}
	}
	return img
}
