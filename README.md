wireguard access for k8s
========================

very simple wireguard server for k8s
this merely syncs a config map into wg-quick and nftables


look at charts/wga/values.yaml for the entire config

you would add peers there, pply to k8s and then get their config, including a nice qr code with


` kubectl exec wg-5bf4d5c7d4-hlbcg -- wga print bob `


qrcode is on stderr, config on stdout, so you can pipe it to a file or whatever
