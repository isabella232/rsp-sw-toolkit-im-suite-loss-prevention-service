/* Apache v2 license
*  Copyright (C) <2019> Intel Corporation
*
*  SPDX-License-Identifier: Apache-2.0
 */

package lossprevention

import (
	"fmt"
	"github.com/edgexfoundry/app-functions-sdk-go/appcontext"
	"github.com/sirupsen/logrus"
	"github.com/intel/rsp-sw-toolkit-im-suite-loss-prevention-service/app/config"
	"github.com/intel/rsp-sw-toolkit-im-suite-loss-prevention-service/app/notification"
	"github.com/intel/rsp-sw-toolkit-im-suite-loss-prevention-service/pkg/camera"
	"github.com/intel/rsp-sw-toolkit-im-suite-loss-prevention-service/pkg/sensor"
	"github.com/intel/rsp-sw-toolkit-im-suite-utilities/helper"
)

const (
	moved              = "moved"
	videoFolderPattern = "/recordings/%v_%s_%s"
)

func HandleDataPayload(edgexcontext *appcontext.Context, payload *DataPayload) error {

	for _, tag := range payload.TagEvent {
		if tag.Event != moved {
			logrus.Debugf("skipping non-moved event: epc: %s (sku: %s), event: %s", tag.Epc, tag.ProductID, tag.Event)
			continue
		}
		if len(tag.LocationHistory) < 2 {
			logrus.Debugf("skipping tag with not enough location history: epc: %s (sku: %s)", tag.Epc, tag.ProductID)
			continue
		}

		if !config.AppConfig.SKUFilterRegex.MatchString(tag.ProductID) {
			logrus.Debugf("skipping tag that does not match sku filter: epc: %s (sku: %s), filter: %s", tag.Epc, tag.ProductID, config.AppConfig.SKUFilter)
			continue
		}
		if !config.AppConfig.EPCFilterRegex.MatchString(tag.Epc) {
			logrus.Debugf("skipping tag that does not match epc filter: epc: %s (sku: %s), filter: %s", tag.Epc, tag.ProductID, config.AppConfig.EPCFilter)
			continue
		}

		rsp := sensor.FindByAntennaAlias(tag.LocationHistory[0].Location)
		logrus.Debugf("current: %+v", rsp)
		if rsp == nil || !rsp.IsExitSensor() {
			logrus.Debugf("skipping non-exiting tag: epc: %s (sku: %s)", tag.Epc, tag.ProductID)
			continue
		}

		rsp2 := sensor.FindByAntennaAlias(tag.LocationHistory[1].Location)
		logrus.Debugf("previous: %+v", rsp2)
		if rsp2 == nil || rsp2.IsExitSensor() {
			logrus.Debugf("skipping exiting tag that was exiting before as well: epc: %s (sku: %s)", tag.Epc, tag.ProductID)
			continue
		}

		logrus.Debugf("triggering on exiting tag: epc: %s (sku: %s)", tag.Epc, tag.ProductID)
		go triggerRecord(edgexcontext, &tag)
		// return so we do not keep checking
		return nil
	}

	return nil
}

func triggerRecord(edgexcontext *appcontext.Context, tag *Tag) {
	timestamp := helper.UnixMilliNow()
	folderName := fmt.Sprintf(videoFolderPattern, timestamp, tag.ProductID, tag.Epc)
	logrus.Debugf("recording filename: %s/video%s", folderName, config.AppConfig.VideoOutputExtension)

	if recorded, err := camera.RecordVideoToDisk(config.AppConfig.VideoDevice, float64(config.AppConfig.RecordingDuration), folderName, config.AppConfig.LiveView); err != nil {
		logrus.Warningf("unable to send EdgeX notification: %+v", err)

	} else if recorded {
		format := `
An item was detected leaving. A video clip has been recorded for loss prevention purposes.

 Timestamp: %d
Product ID: %s
       EPC: %s

`
		content := fmt.Sprintf(format, timestamp, tag.ProductID, tag.Epc)

		if err := notification.PostNotification(edgexcontext, content); err != nil {
			logrus.Error(err)
		}

	}
}
