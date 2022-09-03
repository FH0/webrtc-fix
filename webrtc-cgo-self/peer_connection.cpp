#include <iostream>
#include <string>

#include "api/create_peerconnection_factory.h"

#include "peer_connection.h"

using namespace std;
using namespace webrtc;
using namespace rtc;

class DummySetSessionDescriptionObserver
    : public SetSessionDescriptionObserver {
public:
  static DummySetSessionDescriptionObserver *Create() {
    return new RefCountedObject<DummySetSessionDescriptionObserver>();
  }
  virtual void OnSuccess() {}
  virtual void OnFailure(RTCError error) { assert(false); }
};

class DataChannelObserver : public webrtc::DataChannelObserver {
public:
  DataChannelObserver(scoped_refptr<DataChannelInterface> dataChannel)
      : dataChannel(dataChannel) {}

protected:
  void OnStateChange() {
    cout << "OnStateChange: " << dataChannel->state() << endl;

    if (dataChannel->state() == DataChannelInterface::kOpen) {
    } else if (dataChannel->state() == DataChannelInterface::kClosed) {
      delete (this);
    }
  }

  void OnMessage(const DataBuffer &buffer) { dataChannel->Send(buffer); }

private:
  scoped_refptr<DataChannelInterface> dataChannel;

  void doRead() {}
};

class Server : public PeerConnectionObserver,
               public CreateSessionDescriptionObserver {
public:
  scoped_refptr<PeerConnectionInterface> peerConnection;
  unique_ptr<Thread> signalingThread;

  Server() {
    // 创建 peerConnection
    signalingThread = Thread::Create();
    signalingThread->Start();
    PeerConnectionFactoryDependencies dependencies;
    dependencies.signaling_thread = signalingThread.get();
    auto peerConnectionFactory =
        CreateModularPeerConnectionFactory(move(dependencies));
    PeerConnectionInterface::RTCConfiguration configuration;
    PeerConnectionDependencies connectionDependencies(this);
    auto peerConnectionOrError =
        peerConnectionFactory->CreatePeerConnectionOrError(
            configuration, move(connectionDependencies));
    if (!peerConnectionOrError.ok()) {
      cout << "!peerConnectionOrError.ok()" << endl;
      exit(1);
    }
    peerConnection = peerConnectionOrError.MoveValue();
  }

  void start(char *sdpStr) {
    signalingThread->Invoke<void>(RTC_FROM_HERE, [this, sdpStr]() {
      auto sdp = CreateSessionDescription(SdpType::kOffer, sdpStr);
      peerConnection->SetRemoteDescription(
          DummySetSessionDescriptionObserver::Create(), sdp.release());
      PeerConnectionInterface::RTCOfferAnswerOptions options;
      peerConnection->CreateAnswer(this, options);
    });
  }

protected:
  /*
  PeerConnectionObserver
  */

  void OnSignalingChange(PeerConnectionInterface::SignalingState new_state) {
    cout << "OnSignalingChange: "
         << PeerConnectionInterface::AsString(new_state) << endl;
  }

  void OnDataChannel(scoped_refptr<DataChannelInterface> data_channel) {
    cout << "OnDataChannel: " << data_channel->label() << endl;
    auto dataChannelObserver = new ::DataChannelObserver(data_channel);
    data_channel->RegisterObserver(dataChannelObserver);
  }

  void OnNegotiationNeededEvent(uint32_t event_id) {
    cout << "OnNegotiationNeededEvent: " << event_id << endl;
  }

  void
  OnIceConnectionChange(PeerConnectionInterface::IceConnectionState new_state) {
    cout << "OnIceConnectionChange: "
         << PeerConnectionInterface::AsString(new_state) << endl;
  }

  void
  OnConnectionChange(PeerConnectionInterface::PeerConnectionState new_state) {
    cout << "OnConnectionChange: "
         << PeerConnectionInterface::AsString(new_state) << endl;
  }

  void
  OnIceGatheringChange(PeerConnectionInterface::IceGatheringState new_state) {
    cout << "OnIceGatheringChange: "
         << PeerConnectionInterface::AsString(new_state) << endl;
  }

  void OnIceCandidate(const IceCandidateInterface *candidate) {
    cout << "OnIceCandidate" << endl;
  }

  /*
  CreateSessionDescriptionObserver
  */

  void OnSuccess(SessionDescriptionInterface *desc) {
    cout << "OnSuccess" << endl;

    if (desc->GetType() == SdpType::kAnswer) {
      peerConnection->SetLocalDescription(
          DummySetSessionDescriptionObserver::Create(), desc);
      string sdp;
      peerConnection->local_description()->ToString(&sdp);
      OnEchoServerAnswer((char *)sdp.c_str());
    }
  }

  void OnFailure(RTCError error) {
    cout << "OnFailure: " << error.message() << endl;
  }
};

void EchoServer(char *sdp) {
  scoped_refptr<Server> server{new RefCountedObject<Server>()};
  auto serverPointer = server.release();
  serverPointer->start(sdp);
}
