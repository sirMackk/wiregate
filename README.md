# WireGate

## What is it?

With WireGate, you can setup a local WireGuard VPN quickly and easily.

Imagine that you're in a library or airport and want to share data with your colleagues. Security is important for you. WireGate allows you to set up a WireGuard VPN on the local network. All you and your friends need to do is run _one command each_ and share a password out of band.

**WireGate is at v0.0.9.** You might encounter some rough edges.

## How does it work?

WireGate is a wrapper around WireGuard. It automates the manual steps required to create a VPN on a LAN. Here's how it work:

1. Alice starts the WireGate server and gives the password to Bob on a piece of paper.
2. Bob starts the client, which finds the server on the local network and prompts Bob for the password.
3. After Bob enters the password, the Alice's server configures the new WireGuard peer (IP, subnet, public key, etc.).
4. Bob's client configures its WireGuard client.
5. Both computers can talk to each other securely.

## Using

1. Make sure [WireGuard is installed][0] on all computers.
2. Join the same network. In 2020, this likely means the same wifi access point.
3. On the server, run the following command:

```bash
sudo ./wiregate server -interface eth0 -vpn-password "c4tsRule"
# You should see output similar to this:
INFO[2020-09-15T16:05:18-07:00] Generated private WireGuard key and saved to /tmp/WireGatePrivateKey825961041 
INFO[2020-09-15T16:05:18-07:00] Generated public WireGuard key z87jiZiGDBgC2coHm1EbyJgJxr0q68liSS21aqwxax4= 
INFO[2020-09-15T16:05:18-07:00] Created WireGuard interface wg0, bridged to eth0, and started WireGuard server on 192.168.1.134:51820 
INFO[2020-09-15T16:05:21-07:00] Generated TLS cert at /tmp/WireGateCert.pem733823868 and key at /tmp/WireGatePemKey172175531/key.pem 
INFO[2020-09-15T16:05:21-07:00] Starting TLS HTTP Server...                  
INFO[2020-09-15T16:05:21-07:00] Starting MDNS server...                      
INFO[2020-09-15T16:05:21-07:00] Starting registry purger                     
INFO[2020-09-15T16:05:21-07:00] Starting server on address: :38490
```

4. On the client, run the following command:

```bash
sudo ./wiregate client
# Wait for about a second
Found following WireGate services running on local network:
0) Wiregate (192.168.1.134:38490)
Enter number of service you wish to connect to (0-0):
# In this case, enter 0
Enter password:
# Enter the password
INFO[2020-09-15T16:08:07-07:00] Starting heart beat
```

5. On the server, you will begin to see heartbeat log messages that indicate a new client joined the VPN:

```bash
INFO[2020-09-15T16:08:07-07:00] Successfully registered node 10.24.1.26/24 with pubkey itqZxy1VH5NlqGdZvVy02VsJLGqpVlhAoNpXmFKt60E= as requested by 192.168.1.138:56054 
INFO[2020-09-15T16:08:12-07:00] Successfully registered beat for pubkey itqZxy1VH5NlqGdZvVy02VsJLGqpVlhAoNpXmFKt60E= request by 192.168.1.138:56054 
INFO[2020-09-15T16:08:17-07:00] Successfully registered beat for pubkey itqZxy1VH5NlqGdZvVy02VsJLGqpVlhAoNpXmFKt60E= request by 192.168.1.138:56054
```

6. That's it! The two computers can now talk securely.

## Troubleshooting

Run `wg show` on either server and client to see what peer information they have. Especially useful is the `allowed ips` section. When everything is fine on with just two hosts, it should like this:

```bash
# On the server
$ wg show
interface: wg0
  public key: z87jiZiGDBgC2coHm1EbyJgJxr0q68liSS21aqwxax4=
  private key: (hidden)
  listening port: 51820

peer: itqZxy1VH5NlqGdZvVy02VsJLGqpVlhAoNpXmFKt60E=
  endpoint: 192.168.1.138:55904
  allowed ips: 10.24.1.26/32
  latest handshake: 1 minute, 56 seconds ago
  transfer: 820 B received, 764 B sent
```

```bash
# On the client
$ wg show
interface: wg0
  public key: itqZxy1VH5NlqGdZvVy02VsJLGqpVlhAoNpXmFKt60E=
  private key: (hidden)
  listening port: 55904

peer: z87jiZiGDBgC2coHm1EbyJgJxr0q68liSS21aqwxax4=
  endpoint: 192.168.1.134:51820
  allowed ips: 10.24.1.26/32, 10.24.1.1/32
  latest handshake: 1 minute, 52 seconds ago
  transfer: 764 B received, 820 B sent
```

Check if you can ping the client from the server or the server from the client. The server will start with the lowest IP in the range (10.24.1.1 in this case):

```bash
# On the server
$ ping 10.24.1.26
PING 10.24.1.26 (10.24.1.26) 56(84) bytes of data.
64 bytes from 10.24.1.26: icmp_seq=1 ttl=64 time=2.94 ms
64 bytes from 10.24.1.26: icmp_seq=2 ttl=64 time=4.87 ms
```

```bash
# On the client
$ ping 10.24.1.1
PING 10.24.1.1 (10.24.1.1) 56(84) bytes of data.
64 bytes from 10.24.1.1: icmp_seq=1 ttl=64 time=2.28 ms
64 bytes from 10.24.1.1: icmp_seq=2 ttl=64 time=3.12 ms
```


Because WireGuard changes routing settings on the box you're on, it's also useful to use `route` or `ip route` to see those:

```bash
$ ip route
default via 192.168.1.1 dev wlp4s0 proto dhcp metric 600 
10.24.1.0/24 dev wg0 proto kernel scope link src 10.24.1.26 
169.254.0.0/16 dev wlp4s0 scope link metric 1000 
192.168.1.0/24 dev wlp4s0 proto kernel scope link src 192.168.1.138 metric 600
# Everything looks alright here: traffic for 10.24.1.0/24 should be going out the wg0 interface
```

If there's no wg0 route, that's a good place to start investigating.


[0]: https://www.wireguard.com/
