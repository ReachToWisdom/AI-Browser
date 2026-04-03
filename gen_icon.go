//go:build ignore

package main

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

// AI Browser 아이콘 생성 도구
// go run gen_icon.go 로 실행

func main() {
	// 256x256 아이콘 생성
	img := generateIcon(256)

	// PNG 저장
	savePNG("icon.png", img)

	// ICO 파일 생성 (256, 48, 32, 16 크기 포함)
	sizes := []int{256, 48, 32, 16}
	saveICO("icon.ico", sizes)
}

func generateIcon(size int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	cx, cy := float64(size)/2, float64(size)/2
	r := float64(size) / 2

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			fx, fy := float64(x), float64(y)
			dist := math.Sqrt((fx-cx)*(fx-cx) + (fy-cy)*(fy-cy))

			if dist <= r-1 {
				// 배경 그라디언트 (진한 남색 → 파랑)
				t := dist / r
				bg := lerpColor(
					color.RGBA{15, 23, 42, 255},   // 진한 남색
					color.RGBA{30, 58, 138, 255},   // 남색
					t,
				)

				// 뇌/AI 패턴: 4개 빛나는 노드 (4개 AI 상징)
				nodeColors := []color.RGBA{
					{255, 160, 50, 255},  // Claude (오렌지)
					{66, 133, 244, 255},  // Gemini (파랑)
					{16, 163, 127, 255},  // ChatGPT (초록)
					{140, 100, 255, 255}, // Grok (보라)
				}

				// 노드 위치 (사각형 배치)
				nodePositions := []struct{ x, y float64 }{
					{cx - r*0.25, cy - r*0.25}, // 좌상
					{cx + r*0.25, cy - r*0.25}, // 우상
					{cx - r*0.25, cy + r*0.25}, // 좌하
					{cx + r*0.25, cy + r*0.25}, // 우하
				}

				pixel := bg

				// 연결선 그리기 (노드 사이)
				connections := [][2]int{{0, 1}, {1, 3}, {3, 2}, {2, 0}, {0, 3}, {1, 2}}
				for _, conn := range connections {
					p1 := nodePositions[conn[0]]
					p2 := nodePositions[conn[1]]
					lineDist := distToLine(fx, fy, p1.x, p1.y, p2.x, p2.y)
					lineW := r * 0.025
					if lineDist < lineW {
						lineAlpha := 1.0 - lineDist/lineW
						lineColor := color.RGBA{100, 255, 218, uint8(lineAlpha * 120)}
						pixel = blendOver(pixel, lineColor)
					}
				}

				// 노드(빛나는 원) 그리기
				for i, pos := range nodePositions {
					nodeDist := math.Sqrt((fx-pos.x)*(fx-pos.x) + (fy-pos.y)*(fy-pos.y))
					nodeR := r * 0.14

					// 글로우 효과
					glowR := nodeR * 2.5
					if nodeDist < glowR {
						glowAlpha := (1.0 - nodeDist/glowR) * 0.3
						glowColor := nodeColors[i]
						glowColor.A = uint8(glowAlpha * 255)
						pixel = blendOver(pixel, glowColor)
					}

					// 노드 코어
					if nodeDist < nodeR {
						coreAlpha := 1.0 - (nodeDist/nodeR)*0.3
						coreColor := nodeColors[i]
						coreColor.A = uint8(coreAlpha * 255)
						pixel = blendOver(pixel, coreColor)
					}
				}

				// 중앙 "AI" 심볼 (밝은 원)
				centerDist := math.Sqrt((fx-cx)*(fx-cx) + (fy-cy)*(fy-cy))
				centerR := r * 0.12
				if centerDist < centerR {
					cAlpha := (1.0 - centerDist/centerR) * 0.9
					centerColor := color.RGBA{100, 255, 218, uint8(cAlpha * 255)}
					pixel = blendOver(pixel, centerColor)
				}

				// 안티앨리어싱 (원 테두리)
				if dist > r-2 {
					edgeAlpha := r - dist
					pixel.A = uint8(float64(pixel.A) * edgeAlpha)
				}

				img.SetRGBA(x, y, pixel)
			}
		}
	}

	return img
}

// 두 색 보간
func lerpColor(a, b color.RGBA, t float64) color.RGBA {
	return color.RGBA{
		R: uint8(float64(a.R)*(1-t) + float64(b.R)*t),
		G: uint8(float64(a.G)*(1-t) + float64(b.G)*t),
		B: uint8(float64(a.B)*(1-t) + float64(b.B)*t),
		A: uint8(float64(a.A)*(1-t) + float64(b.A)*t),
	}
}

// 알파 블렌딩
func blendOver(dst, src color.RGBA) color.RGBA {
	sa := float64(src.A) / 255
	da := float64(dst.A) / 255
	outA := sa + da*(1-sa)
	if outA == 0 {
		return color.RGBA{}
	}
	return color.RGBA{
		R: uint8((float64(src.R)*sa + float64(dst.R)*da*(1-sa)) / outA),
		G: uint8((float64(src.G)*sa + float64(dst.G)*da*(1-sa)) / outA),
		B: uint8((float64(src.B)*sa + float64(dst.B)*da*(1-sa)) / outA),
		A: uint8(outA * 255),
	}
}

// 점에서 선분까지 거리
func distToLine(px, py, x1, y1, x2, y2 float64) float64 {
	dx, dy := x2-x1, y2-y1
	lenSq := dx*dx + dy*dy
	if lenSq == 0 {
		return math.Sqrt((px-x1)*(px-x1) + (py-y1)*(py-y1))
	}
	t := math.Max(0, math.Min(1, ((px-x1)*dx+(py-y1)*dy)/lenSq))
	projX, projY := x1+t*dx, y1+t*dy
	return math.Sqrt((px-projX)*(px-projX) + (py-projY)*(py-projY))
}

func savePNG(path string, img *image.RGBA) {
	f, _ := os.Create(path)
	defer f.Close()
	png.Encode(f, img)
}

func saveICO(path string, sizes []int) {
	f, _ := os.Create(path)
	defer f.Close()

	// ICO 헤더
	numImages := len(sizes)
	binary.Write(f, binary.LittleEndian, uint16(0))          // 예약
	binary.Write(f, binary.LittleEndian, uint16(1))          // ICO 타입
	binary.Write(f, binary.LittleEndian, uint16(numImages))  // 이미지 수

	// 각 이미지를 PNG로 인코딩
	pngDatas := make([][]byte, numImages)
	for i, size := range sizes {
		img := generateIcon(size)
		var buf bytes.Buffer
		png.Encode(&buf, img)
		pngDatas[i] = buf.Bytes()
	}

	// 디렉토리 엔트리 오프셋 계산
	headerSize := 6 + numImages*16
	offset := headerSize

	for i, size := range sizes {
		w, h := uint8(size), uint8(size)
		if size >= 256 {
			w, h = 0, 0 // 256 이상은 0으로 표시
		}
		binary.Write(f, binary.LittleEndian, w)               // 너비
		binary.Write(f, binary.LittleEndian, h)               // 높이
		binary.Write(f, binary.LittleEndian, uint8(0))        // 컬러 팔레트
		binary.Write(f, binary.LittleEndian, uint8(0))        // 예약
		binary.Write(f, binary.LittleEndian, uint16(1))       // 컬러 플레인
		binary.Write(f, binary.LittleEndian, uint16(32))      // 비트 깊이
		binary.Write(f, binary.LittleEndian, uint32(len(pngDatas[i]))) // 크기
		binary.Write(f, binary.LittleEndian, uint32(offset))  // 오프셋
		offset += len(pngDatas[i])
	}

	// PNG 데이터 쓰기
	for _, data := range pngDatas {
		f.Write(data)
	}

	_ = zlib.NewWriter // import 유지용
	_ = crc32.NewIEEE
}
