wireguard access for k8s
========================

very simple wireguard server for k8s
this merely syncs a config map into wg-quick and nftables


    cat > values.yaml <<EOF
    config: |
      [Interface]
      Address = 172.27.1.200/24
      PrivateKey = GF1XfJZLrDuCuvXy9mB6PUkqM19w93S5sWD3wteVYUU=
      ListenPort = 51820
      
      [Peer]
      PublicKey = mDtPzUAay7AX0Fi76swpy9gpY8TSBknyQ4cX7cP0nXE=
      AllowedIPs = 172.27.1.11/32
    EOF
    helm install wg oci://ghcr.io/kraudcloud/wga --version 0.1.2-chart \
        --namespace access --create-namespace \
        --values values.yaml
