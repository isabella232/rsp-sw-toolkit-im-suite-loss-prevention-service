/* Apache v2 license
*  Copyright (C) <2019> Intel Corporation
*
*  SPDX-License-Identifier: Apache-2.0
 */

package camera

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/intel/rsp-sw-toolkit-im-suite-loss-prevention-service/app/config"
	"github.com/intel/rsp-sw-toolkit-im-suite-utilities/helper"
	"gocv.io/x/gocv"
	"golang.org/x/sync/semaphore"
	"image"
	"image/color"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"time"
)

const (
	cascadeFolder = "/data/haarcascades"

	font          = gocv.FontHersheySimplex
	fontScale     = 0.75
	fontThickness = 2
	textPadding   = 5

	fileMode = 0777
)

var (
	cameraSemaphone = semaphore.NewWeighted(1)

	red    = color.RGBA{255, 0, 0, 0}
	green  = color.RGBA{0, 255, 0, 0}
	blue   = color.RGBA{0, 0, 255, 0}
	orange = color.RGBA{255, 255, 0, 0}
	white  = color.RGBA{255, 255, 255, 0}
	purple = color.RGBA{255, 0, 255, 0}

	debugStatsColor = green

	cascadeFiles []CascadeFile
)

func convertColor(c float64) color.RGBA {
	return color.RGBA{R: uint8(uint32(c) >> 16 & 0xff), G: uint8(uint32(c) >> 8 & 0xff), B: uint8(uint32(c) & 0xff), A: 0}
}

func SetupCascadeFiles() {
	logrus.Debug("SetupCascadeFiles()")

	if config.AppConfig.EnableFaceDetection {
		cascadeFiles = append(cascadeFiles, CascadeFile{
			name:     "face",
			filename: config.AppConfig.FaceDetectionXmlFile,
			drawOptions: DrawOptions{
				annotation: config.AppConfig.FaceDetectionAnnotation,
				color:      convertColor(config.AppConfig.FaceDetectionColor),
				thickness:  2,
			},
			detectParams: DetectParams{
				scale:        1.4,
				minNeighbors: 4,
				flags:        0,
				minScaleX:    0.05,
				minScaleY:    0.05,
				maxScaleX:    0.8,
				maxScaleY:    0.8,
			},
		})
	}

	if config.AppConfig.EnableProfileFaceDetection {
		cascadeFiles = append(cascadeFiles, CascadeFile{
			name:     "profile_face",
			filename: config.AppConfig.ProfileFaceDetectionXmlFile,
			drawOptions: DrawOptions{
				annotation: config.AppConfig.ProfileFaceDetectionAnnotation,
				color:      convertColor(config.AppConfig.ProfileFaceDetectionColor),
				thickness:  2,
			},
			detectParams: DetectParams{
				scale:        1.4,
				minNeighbors: 4,
				flags:        0,
				minScaleX:    0.1,
				minScaleY:    0.1,
				maxScaleX:    0.8,
				maxScaleY:    0.8,
			},
		})
	}

	if config.AppConfig.EnableUpperBodyDetection {
		cascadeFiles = append(cascadeFiles, CascadeFile{
			name:     "upper_body",
			filename: config.AppConfig.UpperBodyDetectionXmlFile,
			drawOptions: DrawOptions{
				color:      convertColor(config.AppConfig.UpperBodyDetectionColor),
				thickness:  2,
				annotation: config.AppConfig.UpperBodyDetectionAnnotation,
			},
			detectParams: DetectParams{
				scale:        1.5,
				minNeighbors: 3,
				flags:        0,
				minScaleX:    0.1,
				minScaleY:    0.1,
				maxScaleX:    0.75,
				maxScaleY:    0.75,
			},
		})
	}

	if config.AppConfig.EnableFullBodyDetection {
		cascadeFiles = append(cascadeFiles, CascadeFile{
			name:     "full_body",
			filename: config.AppConfig.FullBodyDetectionXmlFile,
			drawOptions: DrawOptions{
				color:      convertColor(config.AppConfig.FullBodyDetectionColor),
				thickness:  2,
				annotation: config.AppConfig.FullBodyDetectionAnnotation,
			},
			detectParams: DetectParams{
				scale:        1.4,
				minNeighbors: 2,
				flags:        0,
				minScaleX:    0.1,
				minScaleY:    0.1,
				maxScaleX:    0.6,
				maxScaleY:    0.8,
			},
		})
	}

	if config.AppConfig.EnableEyeDetection {
		cascadeFiles = append(cascadeFiles, CascadeFile{
			name:     "eye",
			filename: config.AppConfig.EyeDetectionXmlFile,
			drawOptions: DrawOptions{
				color:          convertColor(config.AppConfig.EyeDetectionColor),
				thickness:      1,
				renderAsCircle: true,
			},
			detectParams: DetectParams{
				scale:        1.5,
				minNeighbors: 5,
				flags:        0,
				minScaleX:    0.01,
				minScaleY:    0.01,
				maxScaleX:    0.025,
				maxScaleY:    0.025,
			},
		})
	}

	logrus.Debug("Enabled OpenCV Detections: ", cascadeFiles)
	logrus.Debug("SetupCascadeFiles() complete.")
}

// codecToFloat64 returns a float64 representation of FourCC bytes for use with `gocv.VideoCaptureFOURCC`
func codecToFloat64(codec string) float64 {
	if len(codec) != 4 {
		return -1.0
	}
	c1 := []rune(string(codec[0]))[0]
	c2 := []rune(string(codec[1]))[0]
	c3 := []rune(string(codec[2]))[0]
	c4 := []rune(string(codec[3]))[0]
	return float64((c1 & 255) + ((c2 & 255) << 8) + ((c3 & 255) << 16) + ((c4 & 255) << 24))
}

func (recorder *Recorder) Open() error {
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("recovered from panic: %+v", r)
		}
	}()

	logrus.Debug("Open()")
	var err error

	if recorder.webcam, err = gocv.OpenVideoCapture(recorder.videoDevice); err != nil {
		return errors.Wrapf(err, "Error opening video capture device: %+v", recorder.videoDevice)
	}

	// Note: setting the video capture four cc is very important for performance reasons.
	// 		 it should also be set before applying any size or fps configurations.
	if config.AppConfig.VideoCaptureFOURCC != "" {
		recorder.webcam.Set(gocv.VideoCaptureFOURCC, codecToFloat64(config.AppConfig.VideoCaptureFOURCC))
	}
	if recorder.width != 0 {
		recorder.webcam.Set(gocv.VideoCaptureFrameWidth, float64(recorder.width))
	}
	if recorder.height != 0 {
		recorder.webcam.Set(gocv.VideoCaptureFrameHeight, float64(recorder.height))
	}
	if recorder.fps != 0 {
		recorder.webcam.Set(gocv.VideoCaptureFPS, recorder.fps)
	}
	if config.AppConfig.VideoCaptureBufferSize != 0 {
		recorder.webcam.Set(gocv.VideoCaptureBufferSize, float64(config.AppConfig.VideoCaptureBufferSize))
	}

	// load classifier to recognize faces
	for _, cascadeFile := range cascadeFiles {
		classifier := gocv.NewCascadeClassifier()
		if !classifier.Load(cascadeFolder + "/" + cascadeFile.filename) {
			logrus.Errorf("error reading cascade file: %v", cascadeFile.filename)
			continue
		}

		recorder.cascades = append(recorder.cascades, cascadeFile.AsNewCascade(&classifier))
	}

	//caffeModel := "/opt/intel/openvino/models/intel/face-detection-retail-0004/INT8/face-detection-retail-0004.bin"
	//protoModel := "/opt/intel/openvino/models/intel/face-detection-retail-0004/INT8/face-detection-retail-0004.xml"
	//recorder.net = gocv.ReadNet(caffeModel, protoModel)
	//if recorder.net.Empty() {
	//	return fmt.Errorf("error reading network model %v, %v", caffeModel, protoModel)
	//}
	//
	//recorder.net.SetPreferableBackend(gocv.NetBackendType(gocv.NetBackendOpenVINO))
	//recorder.net.SetPreferableTarget(gocv.NetTargetType(gocv.NetTargetCPU))

	// skip the first few frames (sometimes it takes longer to read, which affects the smoothness of the video)
	recorder.webcam.Grab(config.AppConfig.VideoCaptureBufferSize)

	logrus.Debugf("input codec: %s", recorder.webcam.CodecString())

	if err = os.MkdirAll(recorder.outputFolder, fileMode); err != nil {
		return err
	}

	logrus.Debug("Open() completed")
	return nil
}

func (recorder *Recorder) writeThumb(filename string) {
	logrus.Debugf("writing thumbnail image: %s", filename)
	// compute the width based on the aspect ratio
	width := int(float64(config.AppConfig.ThumbnailHeight) * (float64(config.AppConfig.VideoResolutionWidth) / float64(config.AppConfig.VideoResolutionHeight)))
	thumb := gocv.NewMat()
	gocv.Resize(recorder.frame, &thumb, image.Point{width, config.AppConfig.ThumbnailHeight}, 0, 0, gocv.InterpolationLinear)
	go func() {
		gocv.IMWrite(filepath.Join(recorder.outputFolder, filename), thumb)
		safeClose(&thumb)
	}()
}

func (recorder *Recorder) writeFrame(filename string) {
	logrus.Debugf("writing image: %s", filename)
	cloneFrame := recorder.frame.Clone()
	go func(cloneFrame gocv.Mat) {
		gocv.IMWrite(filepath.Join(recorder.outputFolder, filename), cloneFrame)
		safeClose(&cloneFrame)
	}(cloneFrame)
}

func (recorder *Recorder) writeFrameRegion(filename string, region image.Rectangle) {
	logrus.Debugf("writing image region: %s (%+v)", filename, region)
	regionMat := recorder.frame.Region(region)
	cloneFrame := regionMat.Clone()
	go func(cloneFrame gocv.Mat) {
		gocv.IMWrite(filepath.Join(recorder.outputFolder, filename), cloneFrame)
		safeClose(&cloneFrame)
	}(cloneFrame)
}

// transformProcessRect takes a smaller scaled rectangle produced by a processing function and transforms it
// into a rectangle relative to the full original image size
func transformProcessRect(rect image.Rectangle) image.Rectangle {
	return image.Rectangle{
		Min: image.Point{
			X: rect.Min.X * config.AppConfig.ImageProcessScale,
			Y: rect.Min.Y * config.AppConfig.ImageProcessScale,
		},
		Max: image.Point{
			X: rect.Max.X * config.AppConfig.ImageProcessScale,
			Y: rect.Max.Y * config.AppConfig.ImageProcessScale,
		},
	}
}

func (recorder *Recorder) Close() {
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("recovered from panic: %+v", r)
		}
	}()

	logrus.Debug("Close()")

	safeClose(&recorder.frame)
	safeClose(&recorder.processFrame)

	safeClose(recorder.webcam)
	safeClose(recorder.writer)
	for _, cascade := range recorder.cascades {
		safeClose(cascade.classifier)
	}
	//safeClose(&recorder.net)
	if recorder.liveView {
		safeClose(recorder.window)
	}

	logrus.Debug("Close() completed")
}

func safeClose(c io.Closer) {
	if c == nil {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("recovered from panic: %+v", r)
		}
	}()

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.Debugf("closing %v", reflect.TypeOf(c))
	}

	if err := c.Close(); err != nil {
		logrus.Errorf("error while attempting to close %v: %v", reflect.TypeOf(c), err)
	}
}

func SanityCheck() (bool, error) {
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("recovered from panic: %+v", r)
		}
	}()

	logrus.Debug("SanityCheck()")

	SetupCascadeFiles()

	recorded, err := RecordVideoToDisk(config.AppConfig.VideoDevice, 3.0/float64(config.AppConfig.VideoOutputFps), "/tmp", false)
	logrus.Debugf("SanityCheck() complete. Returned: %v, %+v", recorded, err)
	return recorded, err
}

func RecordVideoToDisk(videoDevice string, seconds float64, outputFolder string, liveView bool) (bool, error) {
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("recovered from panic: %+v", r)
		}
	}()

	// only allow one recording at a time
	// also we do not want to queue up recordings because they would be at invalid times anyways
	if !cameraSemaphone.TryAcquire(1) {
		logrus.Warn("unable to acquire camera lock, we must already be recording. skipping.")
		return false, nil
	}
	defer cameraSemaphone.Release(1)

	recorder := NewRecorder(videoDevice, outputFolder, liveView)
	if err := recorder.Open(); err != nil {
		logrus.Errorf("error: %v", err)
		return false, err
	}

	defer recorder.Close()

	var err error
	recorder.writer, err = gocv.VideoWriterFile(recorder.outputFilename, recorder.codec, recorder.fps, recorder.width, recorder.height, true)
	if err != nil {
		return false, errors.Wrapf(err, "error opening video writer device: %+v", recorder.outputFilename)
	}

	if recorder.liveView {
		//recorder.window.ResizeWindow(1920, 1080)
		recorder.window.ResizeWindow(recorder.width, recorder.height)

		if config.AppConfig.FullscreenView {
			recorder.window.SetWindowProperty(gocv.WindowPropertyFullscreen, gocv.WindowFullscreen)
		}
	}
	begin := time.Now()

	recorder.frameCount = int(math.Round(recorder.fps * seconds))
	var rects []image.Rectangle
	// for debug stats
	var read, process, total DebugStats
	var prevMillis, currentMills, startTS, readTS, processedTS int64
	// pre-compute the x location of the avg stats
	x2 := gocv.GetTextSize("Avg Process: 99.9", font, fontScale, fontThickness).X
	x3 := gocv.GetTextSize("Min Process: 99", font, fontScale, fontThickness).X + x2 + 60
	yPadding := 35
	yStart := 0

	for i := 0; i < recorder.frameCount; i++ {

		startTS = helper.UnixMilliNow()

		if ok := recorder.webcam.Read(&recorder.frame); !ok {
			return false, fmt.Errorf("unable to read from webcam. device closed: %+v", recorder.videoDevice)
		}
		readTS = helper.UnixMilliNow()

		if recorder.frame.Empty() {
			logrus.Debug("skipping empty frame from webcam")
			continue
		}

		if err := recorder.writer.Write(recorder.frame); err != nil {
			logrus.Errorf("error occurred while writing video to disk: %v", err)
		}

		switch i {
		case 0:
			recorder.writeFrame("frame.first.jpg")
			recorder.writeThumb("thumb.jpg")
		case recorder.frameCount / 2:
			recorder.writeFrame("frame.middle.jpg")
		case recorder.frameCount - 1:
			recorder.writeFrame("frame.last.jpg")
		default:
			break
		}

		if len(recorder.cascades) > 0 {
			// Resize smaller for use with the cascade classifiers
			gocv.Resize(recorder.frame, &recorder.processFrame, image.Point{}, 1.0/float64(config.AppConfig.ImageProcessScale), 1.0/float64(config.AppConfig.ImageProcessScale), gocv.InterpolationLinear)

			recorder.overlays = nil
			for _, cascade := range recorder.cascades {
				params := cascade.detectParams

				if reflect.DeepEqual(params, DetectParams{}) {
					rects = cascade.classifier.DetectMultiScale(recorder.processFrame)
				} else {
					rects = cascade.classifier.DetectMultiScaleWithParams(recorder.processFrame, params.scale, params.minNeighbors, params.flags,
						image.Point{X: int(float64(recorder.width) * params.minScaleX), Y: int(float64(recorder.height) * params.minScaleY)},
						image.Point{X: int(float64(recorder.width) * params.maxScaleX), Y: int(float64(recorder.height) * params.maxScaleY)})
				}

				if len(rects) > 0 {
					if cascade.found < len(rects) {
						cascade.found = len(rects)
						logrus.Debugf("Detected %v %s(s)", len(rects), cascade.name)

						if config.AppConfig.SaveObjectDetectionsToDisk {
							for i, rect := range rects {
								recorder.writeFrameRegion(fmt.Sprintf("%s.%d.jpg", cascade.name, i+cascade.written), transformProcessRect(rect))
							}
							// this keeps track of how many we have written before. so if we see 1 face and write it, then see 2 faces, it will not overwrite the first face found
							cascade.written += cascade.found
						}
					} else {
						logrus.Tracef("Detected %v %s(s)", len(rects), cascade.name)
					}

					if liveView {
						for _, rect := range rects {
							recorder.overlays = append(recorder.overlays, FrameOverlay{rect: transformProcessRect(rect), drawOptions: cascade.drawOptions})
						}
					}
				}
			}
		}

		processedTS = helper.UnixMilliNow()

		if recorder.liveView {
			if config.AppConfig.ShowVideoDebugStats {
				read.AddValue(float64(readTS - startTS))
				process.AddValue(float64(processedTS - readTS))
				currentMills = helper.UnixMilliNow()
				if prevMillis != 0 {
					total.AddValue(float64(currentMills - prevMillis))
				}
				prevMillis = currentMills

				// Instant
				gocv.PutText(&recorder.frame, "   Read: "+strconv.FormatInt(int64(read.current), 10),
					image.Point{textPadding, yStart + (yPadding * 1)}, font, fontScale, debugStatsColor, fontThickness)
				gocv.PutText(&recorder.frame, "Process: "+strconv.FormatInt(int64(process.current), 10),
					image.Point{textPadding, yStart + (yPadding * 2)}, font, fontScale, debugStatsColor, fontThickness)
				gocv.PutText(&recorder.frame, "    FPS: "+strconv.FormatFloat(total.FPS(), 'f', 1, 64),
					image.Point{textPadding, yStart + (yPadding * 3)}, font, fontScale, debugStatsColor, fontThickness)

				// Min / Max
				gocv.PutText(&recorder.frame, "   Min Read: "+strconv.FormatInt(int64(read.min), 10),
					image.Point{x2, yStart + (yPadding * 1)}, font, fontScale, debugStatsColor, fontThickness)
				gocv.PutText(&recorder.frame, "   Max Read: "+strconv.FormatInt(int64(read.max), 10),
					image.Point{x2, yStart + (yPadding * 2)}, font, fontScale, debugStatsColor, fontThickness)
				gocv.PutText(&recorder.frame, "Min Process: "+strconv.FormatInt(int64(process.min), 10),
					image.Point{x2, yStart + (yPadding * 3)}, font, fontScale, debugStatsColor, fontThickness)
				gocv.PutText(&recorder.frame, "Max Process: "+strconv.FormatInt(int64(process.max), 10),
					image.Point{x2, yStart + (yPadding * 4)}, font, fontScale, debugStatsColor, fontThickness)

				// Average
				gocv.PutText(&recorder.frame, "   Avg Read: "+strconv.FormatFloat(read.Average(), 'f', 1, 64),
					image.Point{x3, yStart + (yPadding * 1)}, font, fontScale, debugStatsColor, fontThickness)
				gocv.PutText(&recorder.frame, "Avg Process: "+strconv.FormatFloat(process.Average(), 'f', 1, 64),
					image.Point{x3, yStart + (yPadding * 2)}, font, fontScale, debugStatsColor, fontThickness)
				gocv.PutText(&recorder.frame, "    Avg FPS: "+strconv.FormatFloat(total.AverageFPS(), 'f', 1, 64),
					image.Point{x3, yStart + (yPadding * 3)}, font, fontScale, debugStatsColor, fontThickness)

			}

			for _, overlay := range recorder.overlays {
				if overlay.drawOptions.renderAsCircle {
					radius := (overlay.rect.Max.X - overlay.rect.Min.X) / 2
					gocv.Circle(&recorder.frame, image.Point{overlay.rect.Max.X - radius, overlay.rect.Max.Y - radius}, radius, overlay.drawOptions.color, overlay.drawOptions.thickness)
				} else {
					gocv.Rectangle(&recorder.frame, overlay.rect, overlay.drawOptions.color, overlay.drawOptions.thickness)
				}
				gocv.PutText(&recorder.frame, overlay.drawOptions.annotation, image.Point{overlay.rect.Min.X, overlay.rect.Min.Y - 10}, font, 1, overlay.drawOptions.color, fontThickness)
			}

			recorder.window.IMShow(recorder.frame)
			key := recorder.window.WaitKey(1)

			// ESC, Q, q
			if key == 27 || key == 'q' || key == 'Q' {
				logrus.Debugf("stopping video live view")
				recorder.liveView = false
				safeClose(recorder.window)
			}
		}
	}

	logrus.Debugf("recording took %v", time.Now().Sub(begin))

	return true, nil
}
