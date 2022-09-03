#ifndef PEER_CONNECTION_H
#define PEER_CONNECTION_H

#ifdef __cplusplus
extern "C" {
#endif

void EchoServer(char *sdp);
void OnEchoServerAnswer(char *sdp);

#ifdef __cplusplus
}
#endif

#endif /* PEER_CONNECTION_H */
