#!/bin/bash

set -e

args="$@"

# NOTE: This script is only intended to be run within the builder docker container, NOT natively

printf "\e[36mSetting up OpenVINO environment...\e[0m"
. /opt/intel/openvino/bin/setupvars.sh > /dev/null
printf "\e[32m [OK]\e[0m\n"

printf "\e[36mSetting up GoCV build environment...\e[0m"
export CGO_CXXFLAGS="--std=c++11"
export CGO_CPPFLAGS="-I${INTEL_OPENVINO_DIR}/opencv/include -I${INTEL_OPENVINO_DIR}/deployment_tools/inference_engine/include"
export LIBRARY_PATH=${LD_LIBRARY_PATH}
export CGO_LDFLAGS="-lpthread -ldl -lopencv_core -lopencv_videoio -lopencv_imgproc -lopencv_highgui -lopencv_imgcodecs -lopencv_objdetect -lopencv_features2d -lopencv_video -lopencv_dnn -lopencv_calib3d"
printf "\e[32m [OK]\e[0m\n"

printf "\e[33mRunning Unit Tests...\e[0m\n"
GOOS=linux GOARCH=amd64 CGO_ENABLED=1 GO111MODULE=auto go test ${args} -v -tags openvino