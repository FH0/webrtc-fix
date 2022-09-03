package main

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"github.com/pion/webrtc/v3"
)

var startTime time.Time

/*
go run ./webrtc-go-self
*/
func main() {
	offerChan := make(chan string)
	answerChan := make(chan string)
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
	})
	peerConnection.OnICECandidate(func(i *webrtc.ICECandidate) {
	})

	// 接收 offer
	offerJSON := <-offerRecvChan
	var offer webrtc.SessionDescription
	err = json.Unmarshal([]byte(offerJSON), &offer)
	if err != nil {
		panic(err)
	}
	err = peerConnection.SetRemoteDescription(offer)
	if err != nil {
		panic(err)
	}

	// 发送 answer
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		panic(err)
	}
	peerConnection.SetLocalDescription(answer)
	answerJSON, err := json.Marshal(*peerConnection.LocalDescription())
	if err != nil {
		panic(err)
	}
	answerSendChan <- string(answerJSON)

	// 从 dataChannel 接收数据然后原样转发回去
	peerConnection.OnDataChannel(func(dc *webrtc.DataChannel) {
		dc.OnClose(func() {
			fmt.Println("dc.OnClose")
		})
		dc.OnOpen(func() {
			fmt.Printf("Data channel '%s'-'%d' open.\n", dc.Label(), dc.ID())
		})
		dc.OnMessage(func(msg webrtc.DataChannelMessage) {
			err = dc.Send(msg.Data)
			if err != nil {
				panic(err)
			}
		})
	})
}
