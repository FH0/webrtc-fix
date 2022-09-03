package main

/*
#include "peer_connection.h"
#include <stdlib.h>
*/
import "C"

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/pion/webrtc/v3"
)

var (
	offerChan  = make(chan string)
	answerChan = make(chan string)
)
var startTime time.Time

/*
CC=x86_64-linux-gnu-gcc \
	CXX=x86_64-linux-gnu-g++ \
	CGO_CXXFLAGS="-I/root/go/src/github.com/isrc-cas/gt/dep/_webrtc-native/src -I/root/go/src/github.com/isrc-cas/gt/dep/_webrtc-native/src/third_party/abseil-cpp -std=c++17 -DWEBRTC_POSIX -O2" \
	CGO_LDFLAGS="/root/go/src/github.com/isrc-cas/gt/dep/_webrtc-native/src/out/release-x86_64-linux-gnu/obj/libwebrtc.a -ldl -pthread" \
	go run ./webrtc-cgo-self
*/
func main() {
	client(10*1024, 1024, offerChan, answerChan)
	echoServer(offerChan, answerChan)
	select {}
}

func client(packetNum, packetSize int, offerSendChan, answerRecvChan chan string) {
	// 创建 peerConnection
	config := webrtc.Configuration{}
	var err error
	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}

	// 注册 peerConnection 回调
	peerConnection.OnICEConnectionStateChange(func(is webrtc.ICEConnectionState) {
		fmt.Println("peerConnection.OnICEConnectionStateChange: ", is.String())
	})
	peerConnection.OnNegotiationNeeded(func() {
		fmt.Println("peerConnection.OnNegotiationNeeded")

		offer, err := peerConnection.CreateOffer(nil)
		if err != nil {
			panic(err)
		}
		err = peerConnection.SetLocalDescription(offer)
		if err != nil {
			panic(err)
		}
	})
	peerConnection.OnICECandidate(func(i *webrtc.ICECandidate) {
		// 跳过，直到最后一次
		if i != nil {
			return
		}

		// 发送 offer
		offerJSON, err := json.Marshal(*peerConnection.LocalDescription())
		if err != nil {
			panic(err)
		}
		offerSendChan <- string(offerJSON)

		// 接收 answer
		answerJSON := <-answerRecvChan
		var answer webrtc.SessionDescription
		err = json.Unmarshal([]byte(answerJSON), &answer)
		if err != nil {
			panic(err)
		}
		err = peerConnection.SetRemoteDescription(answer)
		if err != nil {
			panic(err)
		}
	})

	// 创建 dataChannel
	dataChan, err := peerConnection.CreateDataChannel("echo test", nil)
	if err != nil {
		panic(err)
	}
	dataChan.OnClose(func() {
		fmt.Println("dataChan.OnClose")
		fmt.Println(time.Since(startTime))
		os.Exit(0)
	})
	buf := make([]byte, packetSize)
	_, err = rand.Read(buf)
	if err != nil {
		panic(err)
	}
	dataChan.OnOpen(func() {
		fmt.Printf("Data channel '%s'-'%d' open.\n", dataChan.Label(), dataChan.ID())
		startTime = time.Now()

		for i := 0; i < packetNum; i++ {
			dataChan.Send(buf)
			if err != nil {
				panic(err)
			}
		}
	})
	var packetCount uint32
	dataChan.OnMessage(func(msg webrtc.DataChannelMessage) {
		if !bytes.Equal(buf, msg.Data) {
			panic("!bytes.Equal(buf, msg.Data)")
		}

		if atomic.AddUint32(&packetCount, 1) == uint32(packetNum) {
			err = dataChan.Close()
			if err != nil {
				panic(err)
			}
		}
	})
}

func echoServer(offerRecvChan, answerSendChan chan string) {
	offerJSON := <-offerRecvChan
	var offer webrtc.SessionDescription
	err := json.Unmarshal([]byte(offerJSON), &offer)
	if err != nil {
		panic(err)
	}

	sdp := C.CString(offer.SDP)
	C.EchoServer(sdp)
	C.free(unsafe.Pointer(sdp))
}

//export OnEchoServerAnswer
func OnEchoServerAnswer(sdpC *C.char) {
	sdp := C.GoString(sdpC)
	answer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  sdp,
	}
	answerJSON, err := json.Marshal(&answer)
	if err != nil {
		panic(err)
	}
	answerChan <- string(answerJSON)
}
