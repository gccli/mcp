# Jenkins VIP ARP Troubleshooting

## Summary

Jenkins could not be accessed from other networks through `http://jenkins.0a3a9012.sslip.io` even though DNS resolved correctly.

The root cause was not DNS, Gateway, or HTTPRoute configuration. The real issue was that the Jenkins Envoy Gateway pod was scheduled onto a node with the label `node.kubernetes.io/exclude-from-external-load-balancers`, which prevented MetalLB from announcing the VIP at L2. As a result, the VIP `10.58.144.18` had no ARP responder and `ip neigh` showed `FAILED`.

After removing `node.kubernetes.io/exclude-from-external-load-balancers` from the selected node, the VIP became reachable and the issue was resolved.

## Symptoms

- `jenkins.0a3a9012.sslip.io` resolved to `10.58.144.18`
- Jenkins Gateway showed `Programmed=True`
- Jenkins HTTPRoute was attached and valid
- `curl` from networks outside the local node failed
- `ip neigh | grep 10.58.144.18` returned `FAILED`
- Another VIP, `10.58.144.16`, worked and had a learned MAC address

Example:

```bash
ip neigh | grep '10.58.144.18\|10.58.144.16'

10.58.144.16 dev br0 lladdr dc:99:14:01:a3:71 STALE
10.58.144.18 dev br0 FAILED
```

## Investigation Path

### 1. Confirm DNS was correct

```bash
nslookup jenkins.0a3a9012.sslip.io
```

Result:

```text
jenkins.0a3a9012.sslip.io -> 10.58.144.18
```

This showed `sslip.io` was doing its job. The hostname correctly mapped to the VIP.

### 2. Confirm Gateway and Route were programmed

```bash
kubectl -n jenkins get gateway,httproute -o wide
```

Result:

- Gateway address: `10.58.144.18`
- Gateway status: `Programmed=True`
- HTTPRoute hostname matched the Jenkins domain

This ruled out basic Gateway API configuration errors.

### 3. Compare with a known-good VIP

Working VIP:

- `10.58.144.16`
- Backed by `cloudagent`
- Had a MAC in `ip neigh`

Broken VIP:

- `10.58.144.18`
- Backed by Jenkins Gateway
- No MAC in `ip neigh`

This strongly suggested an L2 announcement problem rather than an HTTP or DNS problem.

### 4. Check MetalLB and service exposure details

```bash
kubectl get svc -A -o wide | grep '10.58.144.16\|10.58.144.18'
kubectl get gateway -A -o wide | grep '10.58.144.16\|10.58.144.18'
```

Both VIPs were allocated from the same MetalLB pool.

The Jenkins LoadBalancer service was configured with:

```text
externalTrafficPolicy: Local
```

That meant only a node with a local backend endpoint could safely announce the VIP.

### 5. Check MetalLB L2Advertisement constraints

```bash
kubectl get ipaddresspool,l2advertisement -A -o yaml
```

Important detail:

```yaml
spec:
  interfaces:
    - br0
  ipAddressPools:
    - vips
  nodeSelectors:
    - matchLabels:
        extif: br0
```

This meant only nodes labeled `extif=br0` were eligible to announce VIPs on `br0`.

### 6. Initial root cause candidate

At first, Jenkins Envoy was scheduled on `vm60.example.com`, which had no `extif=br0` label. That made the initial failure easy to explain:

- Jenkins service used `externalTrafficPolicy: Local`
- Jenkins Envoy pod was on `vm60`
- `vm60` was not eligible for L2 announcement
- Other eligible nodes had no local endpoint
- Therefore nobody announced `10.58.144.18`

To address that, an `EnvoyProxy` was added so Jenkins Envoy would schedule onto nodes labeled `extif=br0`.

## Why Adding `EnvoyProxy` Was Not Sufficient

After adding `EnvoyProxy`, the Jenkins Envoy pod moved from `vm60.example.com` to `n1.example.com`.

That solved one constraint:

- `n1` had `extif=br0`

But the VIP still did not work.

Further inspection showed:

```bash
kubectl get node n1.example.com --show-labels
```

`n1.example.com` had this additional label:

```text
node.kubernetes.io/exclude-from-external-load-balancers=
```

That label caused the real remaining problem.

## Final Root Cause

The Jenkins Envoy pod ended up on a node that matched the L2Advertisement selector, but that same node was marked with:

```text
node.kubernetes.io/exclude-from-external-load-balancers
```

MetalLB did not create a `ServiceL2Status` for the Jenkins service on that node, and the VIP `10.58.144.18` was never announced.

Evidence:

- Jenkins Envoy pod had a local endpoint on `n1.example.com`
- `ip neigh` still showed `10.58.144.18 FAILED`
- `kubectl get servicel2statuses -A -o yaml` showed entries for working services, but not for Jenkins
- `n1.example.com` carried the external-load-balancer exclusion label

This meant the data path was blocked before HTTP traffic even reached Envoy.

## Fix

Remove the label from the selected node:

```bash
kubectl label node n1.example.com node.kubernetes.io/exclude-from-external-load-balancers-
```

After that:

- MetalLB was able to announce `10.58.144.18`
- ARP resolution succeeded
- Jenkins became reachable from other networks

## Key Commands Used During Debugging

### Check DNS

```bash
nslookup jenkins.0a3a9012.sslip.io
```

### Check Gateway and routes

```bash
kubectl -n jenkins get gateway,httproute -o wide
kubectl -n jenkins get gateway jenkins -o yaml
```

### Check VIP ownership

```bash
kubectl get svc -A -o wide | grep '10.58.144.16\|10.58.144.18'
```

### Check neighbor table

```bash
ip neigh | grep '10.58.144.18\|10.58.144.16'
```

### Check MetalLB configuration

```bash
kubectl get ipaddresspool,l2advertisement -A -o yaml
kubectl get servicel2statuses -A -o yaml
```

### Check actual Envoy pod placement

```bash
kubectl -n envoy-gateway-system get deploy,pod -o wide | grep envoy-jenkins
kubectl -n envoy-gateway-system get svc,endpoints envoy-jenkins-jenkins-865e4b27 -o yaml
```

### Check node labels

```bash
kubectl get nodes --show-labels
kubectl get node n1.example.com --show-labels
```

## Lessons Learned

1. `sslip.io` only solves DNS.

It can map a hostname to a private VIP, but it does not make a private network reachable.

2. `Gateway Programmed=True` does not guarantee network reachability.

Gateway API status can be healthy while L2 advertisement is still broken.

3. `externalTrafficPolicy: Local` adds a hard placement constraint.

Only nodes with local endpoints can be used for advertisement, so pod placement matters.

4. MetalLB eligibility is the intersection of multiple rules.

A node must satisfy all relevant conditions:

- allowed by `L2Advertisement.nodeSelectors`
- able to advertise on the configured interface
- have a local endpoint when `externalTrafficPolicy=Local`
- not be excluded from external load balancers

5. `ip neigh` is a fast signal for L2 problems.

If the VIP is in `FAILED` state while a similar VIP has a MAC, investigate ARP/L2 advertisement before debugging HTTP.

## Recommended Long-Term Improvements

1. Avoid relying on generic labels like `extif=br0` alone.

Use a dedicated scheduling label such as `gateway=public` or `vip=enabled` so Gateway data plane pods land only on intended nodes.

2. Keep control-plane node exposure decisions explicit.

If a node should not carry public VIPs, keep `node.kubernetes.io/exclude-from-external-load-balancers`. If it should carry them, remove the label intentionally and document why.

3. When using `EnvoyProxy`, align it with MetalLB policy.

Pod scheduling, MetalLB node selectors, and external traffic policy must be designed together.

4. Add a standard validation checklist for new Gateways.

For each new Gateway VIP, validate:

- DNS resolution
- Gateway status
- Envoy pod placement
- Service endpoints
- ServiceL2Status presence
- ARP resolution with `ip neigh`

## Related Configuration Change

Jenkins manifest was updated to add an `EnvoyProxy` and bind the `Gateway` to it so the Envoy data plane can be scheduled with a node selector.

This solved the initial placement issue, but the final working fix still depended on removing the external LB exclusion label from the selected node.
