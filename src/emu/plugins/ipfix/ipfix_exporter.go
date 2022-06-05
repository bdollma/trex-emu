package ipfix

import (
	"emu/core"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/intel-go/fastjson"
)

const (
	defaultFileExporterName           = "fnf_agg.ipfix"
	defaultFileExporterMaxSize        = 1048576
	defaultFileExporterMaxIntervalSec = 60
	defaultFileExporterCompress       = true
	defaultFileExporterDir            = ""
	defaultFileExporterMaxFiles       = 100
)

var (
	ErrExporterWrongKernelMode error = errors.New("Failed to create exporter - wrong kernel mode")
)

// Interface type for exporters
type Exporter interface {
	Write(b []byte) (n int, err error)
	Close() error
	GetMaxSize() int
	GetType() string
	GetCountersDbVec() *core.CCounterDbVec
	GetInfoJson() interface{}
	// Indicates whether the exporter relies on kernel level IO (sockets or files)
	GetKernelMode() bool
}

func CreateExporter(client *PluginIPFixClient, dstUrl *url.URL, initJson *fastjson.RawMessage) (Exporter, error) {
	if client == nil || dstUrl == nil {
		return nil, errors.New("Failed to create exporter - client or dstUrl are nil")
	}

	creator := exporterCreators[dstUrl.Scheme]
	exporter, err := creator(client, dstUrl, initJson)

	return exporter, err
}

type exporterCreatorFunc func(client *PluginIPFixClient, dstUrl *url.URL, initJson *fastjson.RawMessage) (Exporter, error)

var exporterCreators = map[string]exporterCreatorFunc{
	"emu-udp": createEmuUdpExporter,
	"udp":     createUdpExporter,
	"file":    createFileExporter,
	"http":    createHttpExporter,
	"https":   createHttpExporter}

func createEmuUdpExporter(client *PluginIPFixClient, dstUrl *url.URL, initJson *fastjson.RawMessage) (Exporter, error) {
	if dstUrl.Scheme != "emu-udp" {
		return nil, errors.New("Invalid dst URL scheme used to create file exporter (should be emu-udp)")
	}

	emuUdpExporter, err := NewEmuUdpExporter(dstUrl.Host, client.Client, client)
	if err != nil {
		return nil, err
	}

	return emuUdpExporter, nil
}

func createUdpExporter(client *PluginIPFixClient, dstUrl *url.URL, initJson *fastjson.RawMessage) (Exporter, error) {
	if dstUrl.Scheme != "udp" {
		return nil, errors.New("Invalid dst URL scheme used to create file exporter (should be udp)")
	}

	udpExporter, err := NewUdpExporter(client, dstUrl.Host)
	if err != nil {
		return nil, err
	}

	return udpExporter, nil
}

func createFileExporter(client *PluginIPFixClient, dstUrl *url.URL, initJson *fastjson.RawMessage) (Exporter, error) {
	if dstUrl.Scheme != "file" {
		return nil, errors.New("Invalid dst URL scheme used to create file exporter (should be file)")
	}

	params := &FileExporterParams{
		Name:        defaultFileExporterName,
		MaxSize:     defaultFileExporterMaxSize,
		MaxInterval: defaultFileExporterMaxIntervalSec,
		Compress:    defaultFileExporterCompress,
		Dir:         defaultFileExporterDir,
		MaxFiles:    defaultFileExporterMaxFiles,
	}

	if len(dstUrl.Path) > 0 {
		params.Name = filepath.Base(dstUrl.Path)
		params.Dir = filepath.Dir(dstUrl.Path)
	}

	if initJson != nil {
		err := client.Tctx.UnmarshalValidate(*initJson, params)
		if err != nil {
			return nil, err
		}
	}

	// Convert from seconds to ns (time.Duration) as expected by FileExporter module
	params.MaxInterval = params.MaxInterval * time.Second

	// Create unique directory per EMU client in the user's dir
	params.Dir = getClientDirExporterName(params.Dir, params.Name, client.Client)

	fileExporter, err := NewFileExporter(client, params)
	if err != nil {
		return nil, err
	}

	return fileExporter, nil
}

func createHttpExporter(client *PluginIPFixClient, dstUrl *url.URL, initJson *fastjson.RawMessage) (Exporter, error) {
	if dstUrl.Scheme != "http" && dstUrl.Scheme != "https" {
		return nil, errors.New("Invalid dst URL scheme used to create HTTP/HTTPS exporter (should be http or https)")
	}

	params := &HttpExporterParams{
		Name:        defaultFileExporterName,
		MaxSize:     defaultFileExporterMaxSize,
		MaxInterval: defaultFileExporterMaxIntervalSec,
		Compress:    defaultFileExporterCompress,
		Dir:         defaultFileExporterDir,
		MaxFiles:    defaultFileExporterMaxFiles,
	}

	params.Url = dstUrl.String()

	if initJson != nil {
		err := client.Tctx.UnmarshalValidate(*initJson, params)
		if err != nil {
			return nil, err
		}
	}

	// Convert from seconds to ns (time.Duration) as expected by IPFixHttpExporter module
	params.MaxInterval = params.MaxInterval * time.Second

	// Create unique directory per EMU client in the user given dir
	params.Dir = getClientDirExporterName(params.Dir, params.Name, client.Client)

	// The directory used to cache files for the HTTP exporter is removed when the exporter is closed
	params.removeDirOnClose = true

	fileExporter, err := NewHttpExporter(client, params)
	if err != nil {
		return nil, err
	}

	return fileExporter, nil
}

func getClientDirExporterName(dir string, name string, client *core.CClient) string {
	if dir == "" {
		dir = os.TempDir()
	}

	strippedMac := strings.ReplaceAll(client.Mac.String(), ":", "")
	filename := filepath.Base(name)
	ext := filepath.Ext(filename)
	prefix := filename[:len(filename)-len(ext)]
	var dirname string
	if ext == "." {
		dirname = fmt.Sprintf("%s_%s", prefix, strippedMac)
	} else {
		dirname = fmt.Sprintf("%s_%s%s", prefix, strippedMac, ext)
	}

	res := fmt.Sprintf("%s/%s", dir, dirname)

	return res
}
