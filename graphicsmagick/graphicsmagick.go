// Package graphicsmagick provides a GraphicsMagick Image Server.
package graphicsmagick

import (
	"container/list"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pierrre/imageserver"
)

const (
	globalParam   = "graphicsmagick"
	tempDirPrefix = "imageserver_"
)

// Server is a GraphicsMagick Image Server.
//
// It gets the Image from the underlying Server then processes it with the GraphicsMagick command line (mogrify command).
//
// All params are extracted from the "graphicsmagick" node param and are optionals.
//
// See GraphicsMagick documentation for more information about arguments.
//
// Params:
//
// - ignore_ratio: "!" for "-resize" argument
//
// - fill: "^" for "-resize" argument
//
// - only_shrink_larger: ">" for "-resize" argument
//
// - only_enlarge_smaller: "<" for "-resize" argument
//
// - extent: "-extent" param, uses width/height params
//
// - width / height: sizes for "-resize" argument (both optionals)
//
// - quality: "-quality" param
//
// - background: color for "-background" argument, 3/4/6/8 lower case hexadecimal characters
//
// - format: "-format" param
//
// - gravity: "-gravity" param, default is Center
//
// - crop: "-crop" param with "+repage"
//
// - rotate: "-rotate" param
//
// - monochrome: "-monochrome" param
//
// - grey: "-colorspace" param with "GRAY"
//
// - trim: "-trim" param
//
// - no_interlace: "-interlace" param with "Line"
//
// - flip: "-flip" param
//
// - flop: "-flop" param
type Server struct {
	Server         imageserver.Server
	Executable     string        // path to "gm" executable, usually "/usr/bin/gm"
	Timeout        time.Duration // timeout for process, optional
	TempDir        string        // temp directory for image files, optional
	AllowedFormats []string      // allowed format list, optional
}

// Get implements Server.
func (server *Server) Get(params imageserver.Params) (*imageserver.Image, error) {
	im, err := server.Server.Get(params)
	if err != nil {
		return nil, err
	}
	if !params.Has(globalParam) {
		return im, nil
	}
	params, err = params.GetParams(globalParam)
	if err != nil {
		return nil, err
	}
	if params.Empty() {
		return im, nil
	}
	im, err = server.process(im, params)
	if err != nil {
		if err, ok := err.(*imageserver.ParamError); ok {
			err.Param = globalParam + "." + err.Param
		}
		return nil, err
	}
	return im, nil
}

func (server *Server) process(im *imageserver.Image, params imageserver.Params) (*imageserver.Image, error) {
	arguments := list.New()

	width, height, err := server.buildArgumentsResize(arguments, params)
	if err != nil {
		return nil, err
	}

	err = server.buildArgumentsBackground(arguments, params)
	if err != nil {
		return nil, err
	}

	err = server.buildArgumentsGravity(arguments, params)
	if err != nil {
		return nil, err
	}

	err = server.buildArgumentsExtent(arguments, params, width, height)
	if err != nil {
		return nil, err
	}

	err = server.buildArgumentsCrop(arguments, params)
	if err != nil {
		return nil, err
	}

	err = server.buildArgumentsRotate(arguments, params)
	if err != nil {
		return nil, err
	}

	err = server.buildArgumentsMonochrome(arguments, params)
	if err != nil {
		return nil, err
	}

	err = server.buildArgumentsGrey(arguments, params)
	if err != nil {
		return nil, err
	}

	err = server.buildArgumentsTrim(arguments, params)
	if err != nil {
		return nil, err
	}

	err = server.buildArgumentsInterlace(arguments, params)
	if err != nil {
		return nil, err
	}

	err = server.buildArgumentsFlip(arguments, params)
	if err != nil {
		return nil, err
	}

	err = server.buildArgumentsFlop(arguments, params)
	if err != nil {
		return nil, err
	}

	format, formatSpecified, err := server.buildArgumentsFormat(arguments, params, im)
	if err != nil {
		return nil, err
	}

	err = server.buildArgumentsQuality(arguments, params, format)
	if err != nil {
		return nil, err
	}

	if arguments.Len() == 0 {
		return im, nil
	}

	arguments.PushFront("mogrify")

	tempDir, err := ioutil.TempDir(server.TempDir, tempDirPrefix)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempDir)

	file := filepath.Join(tempDir, "image")
	arguments.PushBack(file)

	err = ioutil.WriteFile(file, im.Data, os.FileMode(0600))
	if err != nil {
		return nil, err
	}

	argumentSlice := convertArgumentsToSlice(arguments)
	cmd := exec.Command(server.Executable, argumentSlice...)
	err = server.runCommand(cmd)
	if err != nil {
		return nil, err
	}

	if formatSpecified {
		file = fmt.Sprintf("%s.%s", file, format)
	}
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	im = &imageserver.Image{
		Format: format,
		Data:   data,
	}
	return im, nil
}

func (server *Server) buildArgumentsResize(arguments *list.List, params imageserver.Params) (width int, height int, err error) {
	width, err = getDimension("width", params)
	if err != nil {
		return 0, 0, err
	}
	height, err = getDimension("height", params)
	if err != nil {
		return 0, 0, err
	}
	if width == 0 && height == 0 {
		return 0, 0, nil
	}
	widthString := ""
	if width != 0 {
		widthString = strconv.Itoa(width)
	}
	heightString := ""
	if height != 0 {
		heightString = strconv.Itoa(height)
	}
	resize := fmt.Sprintf("%sx%s", widthString, heightString)
	if params.Has("fill") {
		fill, err := params.GetBool("fill")
		if err != nil {
			return 0, 0, err
		}
		if fill {
			resize = resize + "^"
		}
	}
	if params.Has("ignore_ratio") {
		ignoreRatio, err := params.GetBool("ignore_ratio")
		if err != nil {
			return 0, 0, err
		}
		if ignoreRatio {
			resize = resize + "!"
		}
	}
	if params.Has("only_shrink_larger") {
		onlyShrinkLarger, err := params.GetBool("only_shrink_larger")
		if err != nil {
			return 0, 0, err
		}
		if onlyShrinkLarger {
			resize = resize + ">"
		}
	}
	if params.Has("only_enlarge_smaller") {
		onlyEnlargeSmaller, err := params.GetBool("only_enlarge_smaller")
		if err != nil {
			return 0, 0, err
		}
		if onlyEnlargeSmaller {
			resize = resize + "<"
		}
	}
	arguments.PushBack("-resize")
	arguments.PushBack(resize)
	return width, height, nil
}

func getDimension(name string, params imageserver.Params) (int, error) {
	if !params.Has(name) {
		return 0, nil
	}
	dimension, err := params.GetInt(name)
	if err != nil {
		return 0, err
	}
	if dimension < 0 {
		return 0, &imageserver.ParamError{Param: name, Message: "must be greater than or equal to 0"}
	}
	return dimension, nil
}

func (server *Server) buildArgumentsBackground(arguments *list.List, params imageserver.Params) error {
	if !params.Has("background") {
		return nil
	}
	background, err := params.GetString("background")
	if err != nil {
		return err
	}
	switch len(background) {
	case 3, 4, 6, 8:
	default:
		return &imageserver.ParamError{Param: "background", Message: "length must be equal to 3, 4, 6 or 8"}
	}
	for _, r := range background {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return &imageserver.ParamError{Param: "background", Message: "must only contain characters in 0-9a-f"}
		}
	}
	arguments.PushBack("-background")
	arguments.PushBack(fmt.Sprintf("#%s", background))
	return nil
}

func (server *Server) buildArgumentsExtent(arguments *list.List, params imageserver.Params, width int, height int) error {
	if width == 0 || height == 0 {
		return nil
	}
	if !params.Has("extent") {
		return nil
	}
	extent, err := params.GetBool("extent")
	if err != nil {
		return err
	}
	if extent {
		arguments.PushBack("-extent")
		arguments.PushBack(fmt.Sprintf("%dx%d", width, height))
	}
	return nil
}

func (server *Server) buildArgumentsFormat(arguments *list.List, params imageserver.Params, sourceImage *imageserver.Image) (format string, formatSpecified bool, err error) {
	if !params.Has("format") {
		return sourceImage.Format, false, nil
	}
	format, err = params.GetString("format")
	if err != nil {
		return "", false, err
	}
	if server.AllowedFormats != nil {
		ok := false
		for _, f := range server.AllowedFormats {
			if f == format {
				ok = true
				break
			}
		}
		if !ok {
			return "", false, &imageserver.ParamError{Param: "format", Message: "not allowed"}
		}
	}
	arguments.PushBack("-format")
	arguments.PushBack(format)
	return format, true, nil
}

func (server *Server) buildArgumentsQuality(arguments *list.List, params imageserver.Params, format string) error {
	if !params.Has("quality") {
		return nil
	}
	quality, err := params.GetInt("quality")
	if err != nil {
		return err
	}
	if quality < 0 {
		return &imageserver.ParamError{Param: "quality", Message: "must be greater than or equal to 0"}
	}
	if format == "jpeg" {
		if quality < 0 || quality > 100 {
			return &imageserver.ParamError{Param: "quality", Message: "must be between 0 and 100"}
		}
	}
	arguments.PushBack("-quality")
	arguments.PushBack(strconv.Itoa(quality))
	return nil
}

func (server *Server) buildArgumentsGravity(arguments *list.List, params imageserver.Params) error {
	gravity, _ := params.GetString("gravity")
	var translatedGravity string
	if gravity != "" {
		switch {
		case gravity == "n":
			translatedGravity = "North"
		case gravity == "s":
			translatedGravity = "South"
		case gravity == "e":
			translatedGravity = "East"
		case gravity == "w":
			translatedGravity = "West"
		case gravity == "ne":
			translatedGravity = "NorthEast"
		case gravity == "se":
			translatedGravity = "SouthEast"
		case gravity == "nw":
			translatedGravity = "NorthWest"
		case gravity == "sw":
			translatedGravity = "SouthWest"
		}
		if translatedGravity == "" {
			return &imageserver.ParamError{Param: "gravity", Message: "gravity should n, s, e, w, ne, se, nw or sw"}
		}
	} else {
		// Default gravity is center.
		translatedGravity = "Center"
	}

	arguments.PushBack("-gravity")
	arguments.PushBack(fmt.Sprintf("%s", translatedGravity))
	return nil
}

func (server *Server) buildArgumentsCrop(arguments *list.List, params imageserver.Params) error {
	if !params.Has("crop") {
		return nil
	}
	crop, _ := params.GetString("crop")
	cropArgs := strings.Split(crop, ",")
	cropArgsLen := len(cropArgs)
	if cropArgsLen != 2 && cropArgsLen != 4 {
		return &imageserver.ParamError{Param: "crop", Message: "Invalid crop request, parameters number mismatch"}
	}
	if cropArgsLen == 2 {
		width, _ := strconv.Atoi(cropArgs[0])
		height, _ := strconv.Atoi(cropArgs[1])
		arguments.PushBack("-crop")
		arguments.PushBack(fmt.Sprintf("%dx%d", width, height))
		arguments.PushBack("+repage")
	}
	if cropArgsLen == 4 {
		width, _ := strconv.Atoi(cropArgs[0])
		height, _ := strconv.Atoi(cropArgs[1])
		x, _ := strconv.Atoi(cropArgs[2])
		y, _ := strconv.Atoi(cropArgs[3])
		arguments.PushBack("-crop")
		arguments.PushBack(fmt.Sprintf("%dx%d+%d+%d", width, height, x, y))
		arguments.PushBack("+repage")
	}
	return nil
}

func (server *Server) buildArgumentsRotate(arguments *list.List, params imageserver.Params) error {
	rotate, _ := params.GetInt("rotate")
	if rotate != 0 {
		if rotate < 0 || rotate > 359 {
			return &imageserver.ParamError{Param: "rotate", Message: "Invalid rotate parameter"}
		}

		arguments.PushBack("-rotate")
		arguments.PushBack(strconv.Itoa(rotate))
	}
	return nil
}

func (server *Server) buildArgumentsMonochrome(arguments *list.List, params imageserver.Params) error {
	monochrome, _ := params.GetBool("monochrome")
	if monochrome {
		arguments.PushBack("-monochrome")
	}
	return nil
}

func (server *Server) buildArgumentsGrey(arguments *list.List, params imageserver.Params) error {
	grey, _ := params.GetBool("grey")
	if grey {
		arguments.PushBack("-colorspace")
		arguments.PushBack("GRAY")
	}
	return nil
}

func (server *Server) buildArgumentsTrim(arguments *list.List, params imageserver.Params) error {
	trim, _ := params.GetBool("trim")
	if trim {
		// We must execute trim first. (order of operations)
		arguments.PushFront("-trim")
	}
	return nil
}

func (server *Server) buildArgumentsInterlace(arguments *list.List, params imageserver.Params) error {
	interlace, _ := params.GetBool("no_interlace")
	if !interlace {
		arguments.PushBack("-interlace")
		arguments.PushBack("Line")
	}
	return nil
}

func (server *Server) buildArgumentsFlip(arguments *list.List, params imageserver.Params) error {
	flip, _ := params.GetBool("flip")
	if flip {
		arguments.PushBack("-flip")
	}
	return nil
}

func (server *Server) buildArgumentsFlop(arguments *list.List, params imageserver.Params) error {
	flop, _ := params.GetBool("flop")
	if flop {
		arguments.PushBack("-flop")
	}
	return nil
}

func convertArgumentsToSlice(arguments *list.List) []string {
	argumentSlice := make([]string, 0, arguments.Len())
	for e := arguments.Front(); e != nil; e = e.Next() {
		argumentSlice = append(argumentSlice, e.Value.(string))
	}
	return argumentSlice
}

func (server *Server) runCommand(cmd *exec.Cmd) error {
	err := cmd.Start()
	if err != nil {
		return err
	}
	cmdChan := make(chan error, 1)
	go func() {
		cmdChan <- cmd.Wait()
	}()
	var timeoutChan <-chan time.Time
	if server.Timeout != 0 {
		timeoutChan = time.After(server.Timeout)
	}
	select {
	case err = <-cmdChan:
	case <-timeoutChan:
		cmd.Process.Kill()
		err = fmt.Errorf("timeout after %s", server.Timeout)
	}
	if err != nil {
		return &imageserver.ImageError{Message: fmt.Sprintf("GraphicsMagick command: %s", err)}
	}
	return nil
}
