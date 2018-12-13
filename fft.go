package main

import (
	"fmt"
	"math"
	"math/cmplx"
	"os"

	"github.com/mjibson/go-dsp/wav"
	"gonum.org/v1/gonum/fourier"
	"gonum.org/v1/plot/plotter"
)

func readFloats64(w *wav.Wav, n int) ([]float64, error) {
	d, err := w.ReadSamples(n)
	if err != nil {
		return nil, err
	}
	var f []float64
	switch d := d.(type) {
	case []uint8:
		f = make([]float64, len(d))
		for i, v := range d {
			f[i] = float64(v) / math.MaxUint8
		}
	case []int16:
		f = make([]float64, len(d))
		for i, v := range d {
			f[i] = (float64(v) - math.MinInt16) / (math.MaxInt16 - math.MinInt16)
		}
	case []float32:
		f = make([]float64, len(d))
		for i, v := range d {
			f[i] = float64(v)
		}
	default:
		return nil, fmt.Errorf("wav: unknown type: %T", d)
	}
	return f, nil
}

func toMono(samples []float64, channels int) []float64 {
	if channels == 1 {
		return samples
	}

	size := int(len(samples))
	mono := make([]float64, size/channels)

	for i := 0; i < size; i += channels {
		sum := 0.0
		for ch := 0; ch < channels; ch++ {
			sum += samples[i+ch]
		}
		mono[i/channels] = sum / float64(channels)
	}

	return mono
}
func getFirstChan(samples []float64, channels int) []float64 {
	if channels == 1 {
		return samples
	}

	size := int(len(samples))
	mono := make([]float64, size/channels)
	for i := 0; i < size; i += channels {
		mono[i/channels] = samples[i]
	}
	return mono
}

func arrAbs(a []complex128) []float64 {
	abs := make([]float64, len(a))
	for i := range a {
		abs[i] = cmplx.Abs(a[i])
	}
	return abs
}

func toPlotXY(data []float64) plotter.XYs {
	pts := make(plotter.XYs, len(data))
	for i := range pts {
		pts[i].Y = data[i]
		pts[i].X = float64(i)
	}
	return pts
}

func hamming(l int) []float64 {
	window := make([]float64, l)
	for i := range window {
		window[i] = 0.54 - 0.46*math.Cos(2*math.Pi*(float64(i)/float64((l-1))))
	}
	return window
}

func arrMult(dst, scales []float64) []float64 {
	for i := range scales {
		dst[i] *= scales[i]
	}
	return dst
}

func packToInt32(data []float64, dropRate int) []int32 {
	const shift = 1000000
	size := len(data)
	packed := make([]int32, int(math.Ceil(float64(size)/float64(dropRate))))

	for i, j := 0, 0; i < size; i += dropRate {
		packed[j] = int32(data[i] * shift)
		j++
	}

	return packed
}

func getFFT(path string, fftSize int) [][]int32 {
	f, _ := os.Open(path)

	audio, _ := wav.New(f)
	samples, _ := readFloats64(audio, audio.Samples)

	mono := toMono(samples, int(audio.NumChannels))

	sizeMono := int(len(mono))
	fftStep := fftSize

	chunksLen := int(math.Ceil(float64(sizeMono) / float64(fftStep)))

	fftChuncks := make([][]int32, chunksLen)
	idx := 0

	//calc fft for 1 second of mono audio
	for i := int(0); i < sizeMono; i += fftStep {
		end := i + fftStep
		if end >= sizeMono {
			end = sizeMono
		}

		monoChunk := mono[i:end]

		// Initialize an FFT and perform the analysis.
		fft := fourier.NewFFT(len(monoChunk))
		chunk := fft.Coefficients(nil, monoChunk)

		// chunk := fft.FFTReal(mono[i:end])[1:] // ignore 1st element
		chunkAbs := arrAbs(chunk)
		window := hamming(len(chunk))

		fftChuncks[idx] = packToInt32(arrMult(chunkAbs, window[len(chunkAbs)/2:])[1:len(chunkAbs)/2], 2)
		idx++
	}

	return fftChuncks
}
