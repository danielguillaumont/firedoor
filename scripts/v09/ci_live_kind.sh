#!/usr/bin/env bash
# Firedoor 9F1: Linux CI runner ONLY, disposable kind + Calico acceptance.
# Without --execute it performs no Docker/Kubernetes operations.
set -Eeuo pipefail

if [[ $# -eq 0 ]]; then
  printf '%s\n' 'PLAN: Firedoor v0.9F1 live CI requires --execute.' \
    'A fresh, uniquely named kind cluster will be created and deleted.' \
    'No Terraform, production cluster, or existing kubeconfig is used.'
  exit 0
fi
if [[ $# -ne 1 || "${1}" != '--execute' ]]; then
  echo 'ERROR: only an explicit --execute is supported' >&2
  exit 2
fi

for bin in python3 sha256sum curl go docker kind kubectl; do
  command -v "$bin" >/dev/null 2>&1 || { echo "Missing dependency: $bin" >&2; exit 2; }
done

docker info >/dev/null 2>&1 || { echo 'Docker daemon unavailable' >&2; exit 2; }
repo="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd -P)"
cd -- "$repo"
# Fixture checksums independently seal the original reviewed test inputs.
(
  cd "$repo"
  printf '%s\n' \
    'b0ef9e207db85f005ee2d116399b853788d5ae0f3dedec1e6c70d269598ef62d  infrastructure/kind/v09-calico-kind.yaml' \
    '8083e6576c04993f848bd46fa2ae3ca3a831c68b2b3d36f3fada8c6b70a219a0  infrastructure/kind/v09c1-positive-baseline.yaml' \
    'ac790a7ec55628d871399c884a8e3c205790a3df8ef6b5a4f11d2c7ad12b2bb4  infrastructure/kind/v09c2-trusted-ingress-policy.yaml' \
    'dc2d284b52944b93eae2ee1c53db61921239f50295a7da585bb27200178dfd73  infrastructure/kind/v09d1-weakened-ingress-policy.yaml' \
    | sha256sum --check --status
) || { echo 'Unreviewed fixture change; refusing live CI' >&2; exit 2; }

python3 "$repo/scripts/v09/ci_live_boundary.py" --self-test
nonce="$(python3 -c 'import secrets; print(secrets.token_hex(6))')"
name="firedoor-v09-$nonce"
[[ "$name" =~ ^firedoor-v09-[0-9a-f]{12}$ ]] || { echo 'Invalid cluster name' >&2; exit 2; }
if kind get clusters | grep -Fx -- "$name" >/dev/null; then
  echo 'Generated cluster already exists; refusing to touch it' >&2; exit 2
fi

root="${RUNNER_TEMP:-${TMPDIR:-/tmp}}"
run_dir="$(mktemp -d "$root/firedoor-v09-ci.XXXXXXXX")"
kubeconfig="$run_dir/kubeconfig"
cluster_may_exist=0
cleanup() {
  status=$?
  trap - EXIT INT TERM
  if [[ "$cluster_may_exist" == 1 ]]; then
    echo "CLEANUP: deleting ONLY the generated cluster $name"
    if ! kind delete cluster --name "$name" --kubeconfig "$kubeconfig"; then
      echo 'ERROR: kind deletion failed' >&2
      status=1
    fi
    if kind get clusters | grep -Fx -- "$name" >/dev/null; then
      echo 'ERROR: generated cluster still present' >&2
      status=1
    fi
    if [[ -n "$(docker ps -aq --filter "label=io.x-k8s.kind.cluster=$name")" ]]; then
      echo 'ERROR: generated-cluster containers remain' >&2
      status=1
    fi
  fi
  # The directory was created by mktemp in this invocation only.
  rm -rf -- "$run_dir" || status=1
  if [[ $status -eq 0 ]]; then
    echo 'PASS: Firedoor 9F1 generated cluster and temporary kubeconfig cleaned up'
  fi
  exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

# Build typed Go gates from the checked out branch. No network requests from Go gates.
go build -o "$run_dir/livegate" ./scripts/v09/cmd/livegate
go build -o "$run_dir/driftgate" ./scripts/v09/cmd/driftgate

# Calico manifest is version- and content-pinned. Never apply unverified remote YAML.
calico="$run_dir/calico-v3.33.0.yaml"
curl --fail --show-error --silent --location --retry 3 --max-time 120 \
  'https://raw.githubusercontent.com/projectcalico/calico/v3.33.0/manifests/calico.yaml' -o "$calico"
echo '2de8f47595fb9c41b3f47d7b767a1f8e72ecf84057af834738ff12689a234da5  '"$calico" | sha256sum --check --status || {
  echo 'Calico manifest hash mismatch; refusing cluster operation' >&2; exit 2;
}

echo "CREATE: isolated disposable kind cluster $name"
cluster_may_exist=1
kind create cluster --name "$name" --config "$repo/infrastructure/kind/v09-calico-kind.yaml" \
  --image 'kindest/node:v1.36.4' --kubeconfig "$kubeconfig" --wait 180s
context="kind-$name"
current="$(kubectl --kubeconfig "$kubeconfig" config current-context)"
[[ "$current" == "$context" ]] || { echo 'Unexpected kubeconfig context' >&2; exit 1; }

kubectl --kubeconfig "$kubeconfig" --context "$context" apply -f "$calico"
kubectl --kubeconfig "$kubeconfig" --context "$context" -n kube-system rollout status ds/calico-node --timeout=240s
kubectl --kubeconfig "$kubeconfig" --context "$context" wait --for=condition=Ready nodes --all --timeout=180s
# Node readiness does not imply DNS/Service routing readiness. Require the
# disposable cluster's built-in DNS and kube-proxy before HTTP baseline probes.
kubectl --kubeconfig "$kubeconfig" --context "$context" -n kube-system rollout status deployment/coredns --timeout=240s
kubectl --kubeconfig "$kubeconfig" --context "$context" -n kube-system rollout status daemonset/kube-proxy --timeout=240s
python3 "$repo/scripts/v09/ci_live_boundary.py" \
  --cluster "$name" --kubeconfig "$kubeconfig" --repo "$repo" \
  --livegate "$run_dir/livegate" --driftgate "$run_dir/driftgate"
echo 'PASS: Firedoor 9F1 live Kubernetes security acceptance completed'
