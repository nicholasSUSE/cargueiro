package chart

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"path"
)

// limits bound what is read from an archive, whose content is not trusted.
type limits struct {
	// file is the largest Chart.yaml or values.yaml.
	file int64
	// stream is the most that is read from the decompressed archive, which
	// stops a small archive that expands to something huge.
	stream int64
}

func defaultLimits() limits {
	return limits{
		file:   2 << 20,
		stream: 32 << 20,
	}
}

var errTooLarge = errors.New("the archive is too large once decompressed")

// LoadArchive reads Chart.yaml and values.yaml from a chart archive (.tgz). Only
// the two files at the top of the chart are read, by exact path, in memory; the
// rest of the archive is skipped, including the same names in subcharts.
func LoadArchive(r io.Reader) (*Files, error) {
	return loadArchive(r, defaultLimits())
}

func loadArchive(r io.Reader, lim limits) (*Files, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("the archive is not gzip: %w", err)
	}
	defer func() { _ = gz.Close() }() // nothing to flush: the archive is only read

	tr := tar.NewReader(&cappedReader{r: gz, left: lim.stream})

	var chartData, valuesData []byte
	var haveChart, haveValues bool
	for !haveChart || !haveValues {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read the archive: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}

		var dst *[]byte
		var have *bool
		switch topLevelFile(hdr.Name) {
		case "Chart.yaml":
			dst, have = &chartData, &haveChart
		case "values.yaml":
			dst, have = &valuesData, &haveValues
		default:
			continue
		}
		if *have {
			return nil, fmt.Errorf("%s appears twice in the archive", hdr.Name)
		}

		data, err := io.ReadAll(io.LimitReader(tr, lim.file+1))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", hdr.Name, err)
		}
		if int64(len(data)) > lim.file {
			return nil, fmt.Errorf("%s is larger than %d bytes", hdr.Name, lim.file)
		}
		*dst, *have = data, true
	}

	switch {
	case !haveChart:
		return nil, errors.New("the archive has no Chart.yaml")
	case !haveValues:
		return nil, errors.New("the archive has no values.yaml")
	}

	return newFiles("Chart.yaml", chartData, "values.yaml", valuesData)
}

// topLevelFile returns the file name of an archive entry that is directly
// inside the chart's own directory, such as "fleet/Chart.yaml", and "" for
// any other entry, such as "fleet/charts/sub/Chart.yaml" or "Chart.yaml".
func topLevelFile(name string) string {
	dir, file := path.Split(path.Clean(name))
	if dir == "" || path.Dir(path.Clean(dir)) != "." {
		return ""
	}
	return file
}

// cappedReader fails with errTooLarge once left bytes have been read.
type cappedReader struct {
	r    io.Reader
	left int64
}

func (c *cappedReader) Read(p []byte) (int, error) {
	if c.left <= 0 {
		return 0, errTooLarge
	}
	if int64(len(p)) > c.left {
		p = p[:c.left]
	}
	n, err := c.r.Read(p)
	c.left -= int64(n)
	return n, err
}
