# WireGate

## What is it?

WireGate is a CLI tool that makes creating a local WireGuard VPN simple.

It's useful if you want to create a temporary, secure connection between at least 2 hosts on a network you don't trust. For example, you may want to get some work done on public wifi at a library or airport and share data with your colleagues, but you don't want anyone to intercept your traffic. WireGate steamlines this process so that you and your friends have to run just one command each and share the password out of band–it does the subnet selection and peer adding/removing for you.

**WireGate is at v0.0.9 - it works, but it's got plenty of rough edges**

## How does it work?

WireGate is a wrapper around WireGuard that automates a few manual steps for the simple use case of creating a VPN on a LAN. In a nutshell, here's what it does:
1. Alice starts the WireGate server and gives the password to Bob on a piece of paper.
2. Bob starts the client, which finds the server and prompts Bob for the password.
3. After Bob enters the password, WireGate assigns a new IP to the peer and adds its public key.
4. Alice's and Bob's computers can talk to each other securely now.

## Using

1. Make sure [WireGuard is installed][0] on all computers.
2. Join the same network. In 2020, this likely means the same wifi access point.
3. If you're the server, the following command:

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

4. If you're the client, run the following command:

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

You can now send data securely between all hosts that are part of the VPN.

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

Check if you can ping the client from the server or the server from the client. The server will start with the lowest IP in the range (10.24.1.1 in this case).

Because WireGuard changes routing settings on the box you're on, it's also useful to use `route` or `ip route` to see those:

```bash
$ ip route
default via 192.168.1.1 dev wlp4s0 proto dhcp metric 600 
10.24.1.0/24 dev wg0 proto kernel scope link src 10.24.1.26 
169.254.0.0/16 dev wlp4s0 scope link metric 1000 
192.168.1.0/24 dev wlp4s0 proto kernel scope link src 192.168.1.138 metric 600
# Everything looks alright here: traffic for 10.24.1.0/24 should be going out the wg0 interface
```

If there's no wg0 route, then that's where I would start investigating.


[0]: https://www.wireguard.com/
