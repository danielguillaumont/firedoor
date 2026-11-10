#!/usr/bin/env python3
"""Firedoor 9F1 Linux CI live kind/Calico boundary gate.

This is deliberately limited to a uniquely named, disposable kind cluster.
It uses explicit kubeconfig/context on every Kubernetes request, accepts only
one policy in a fixed namespace, and interprets actual API observations through
Firedoor's compiled Go gates. No Terraform or production integration.
"""
import argparse
import ipaddress
import json
import os
import pathlib
import re
import subprocess
import sys
import tempfile
import time

NS = "firedoor-v09c1"
POLICY = "allow-trusted-to-data"
MARKER = "FIREDOOR_V09C1_DATA_OK"
NAME_RE = re.compile(r"firedoor-v09-[0-9a-f]{12}\Z")
UID_RE = re.compile(r"[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}\Z")
APPROVED = {"podSelector": {"matchLabels": {"app": "data-server"}}, "policyTypes": ["Ingress"], "ingress": [{"from": [{"podSelector": {"matchLabels": {"role": "trusted"}}}], "ports": [{"protocol": "TCP", "port": 8080}]}]}
WEAKENED = {"podSelector": {"matchLabels": {"app": "data-server"}}, "policyTypes": ["Ingress"], "ingress": [{"from": [{"podSelector": {}}], "ports": [{"protocol": "TCP", "port": 8080}]}]}


class GateFailure(RuntimeError):
    pass


def require(ok, msg):
    if not ok:
        raise GateFailure(msg)


def command(args, timeout=120):
    p = subprocess.run(args, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=timeout, check=False)
    return p


def self_test():
    good = {"apiVersion": "networking.k8s.io/v1", "kind": "NetworkPolicyList", "metadata": {"resourceVersion": "5"}, "items": []}
    assert parse_list(good) == []
    for field, changed in [("kind", "PodList"), ("apiVersion", "v1"), ("items", {}), ("metadata", {"continue": "token"})]:
        broken = dict(good)
        broken[field] = changed
        try:
            parse_list(broken)
        except GateFailure:
            pass
        else:
            raise AssertionError("accepted malformed Kubernetes policy list: " + field)
    item = {"metadata": {"namespace": NS, "name": POLICY, "uid": "01234567-abcd-1234-abcd-0123456789ab", "resourceVersion": "12"}, "spec": APPROVED}
    assert parse_list(dict(good, items=[item]))[0]["spec"] == APPROVED
    for mutated in [{**item, "kind": "Pod"}, {**item, "metadata": {**item["metadata"], "namespace": "default"}}, {**item, "spec": None}]:
        try:
            parse_list(dict(good, items=[mutated]))
        except GateFailure:
            pass
        else:
            raise AssertionError("accepted invalid policy item")
    require(APPROVED != WEAKENED, "approved and weakened spec identical")

    routes = {"service": "service-url", "pod_ip": "pod-ip-url"}
    seen = []
    emitted = []
    iteration = [0]
    def settling_probe(client, url):
        seen.append((iteration[0], client, url))
        return (iteration[0] > 0, "not-ready")
    def next_round(_):
        iteration[0] += 1
    verify_positive_baseline(settling_probe, lambda: True, routes, attempts=3, delay=0,
                             sleep=next_round, emit=emitted.append)
    require(len(seen) == 8, "baseline did not require four fresh positives after startup")
    require(any("4/4 HTTP probes passed" in line for line in emitted), "baseline success missing")
    try:
        verify_positive_baseline(lambda client, url: (False, "not-ready"), lambda: True, routes,
                                 attempts=2, delay=0, sleep=lambda _: None, emit=lambda _: None)
    except GateFailure as exc:
        require("positive baseline failed" in str(exc), "unexpected persistent failure reason")
    else:
        raise AssertionError("permanent baseline failure was accepted")
    try:
        verify_positive_baseline(lambda client, url: (client != "edge-client" or url != "service-url", "service not ready"),
                                 lambda: True, routes, attempts=2, delay=0,
                                 sleep=lambda _: None, emit=lambda _: None)
    except GateFailure as exc:
        require("service/edge-client" in str(exc), "three-of-four positives were accepted")
    else:
        raise AssertionError("incomplete positive baseline was accepted")
    policy_checks = [0]
    def policy_changes_during_retry():
        policy_checks[0] += 1
        return policy_checks[0] == 1
    try:
        verify_positive_baseline(lambda client, url: (False, "not ready"), policy_changes_during_retry,
                                 routes, attempts=2, delay=0, sleep=lambda _: None, emit=lambda _: None)
    except GateFailure as exc:
        require("unexpected NetworkPolicy" in str(exc), "unexpected policy on retry was accepted")
    else:
        raise AssertionError("policy drift during settling was accepted")
    from types import SimpleNamespace
    fake = object.__new__(Live)
    fake.kubectl = lambda *args, **kwargs: SimpleNamespace(returncode=0, stdout="WRONG_CONTENT", stderr="")
    try:
        fake.baseline_probe("edge-client", "service-url")
    except GateFailure as exc:
        require("without expected marker" in str(exc), "wrong success content was accepted")
    else:
        raise AssertionError("success without marker was accepted")
    try:
        verify_positive_baseline(lambda client, url: (True, "ok"), lambda: False, routes,
                                 attempts=2, delay=0, sleep=lambda _: None, emit=lambda _: None)
    except GateFailure as exc:
        require("unexpected NetworkPolicy" in str(exc), "unexpected policy was not rejected")
    else:
        raise AssertionError("baseline with unapproved policy was accepted")
    try:
        verify_positive_baseline(lambda client, url: (True, "ok"), lambda: True,
                                 {"service": "service-url"}, attempts=1, delay=0)
    except GateFailure as exc:
        require("service and Pod IP" in str(exc), "missing Pod IP control was accepted")
    else:
        raise AssertionError("missing positive control was accepted")
    print("PASS: Firedoor 9F1 offline parser/identity and fail-closed baseline readiness tests")


def parse_list(doc):
    require(isinstance(doc, dict), "Kubernetes list must be an object")
    typed = (doc.get("kind"), doc.get("apiVersion")) == ("NetworkPolicyList", "networking.k8s.io/v1")
    generic = (doc.get("kind"), doc.get("apiVersion")) == ("List", "v1")
    require(typed or generic, "unknown policy list kind/apiVersion")
    meta = doc.get("metadata")
    require(isinstance(meta, dict) and not meta.get("continue"), "incomplete or malformed policy list metadata")
    items = doc.get("items")
    require(isinstance(items, list), "policy list items must be an array")
    require(len(items) <= 1, "additional policies in isolated namespace")
    for item in items:
        require(isinstance(item, dict), "malformed policy")
        kind, version = item.get("kind"), item.get("apiVersion")
        require(kind in (None, "NetworkPolicy") and version in (None, "networking.k8s.io/v1"), "foreign resource in NetworkPolicy list")
        require(not generic or (kind == "NetworkPolicy" and version == "networking.k8s.io/v1"), "generic list lacks typed item")
        im = item.get("metadata")
        require(isinstance(im, dict) and im.get("namespace") == NS and im.get("name") == POLICY, "wrong policy namespace or name")
        require(bool(UID_RE.fullmatch(str(im.get("uid", "")))), "missing or invalid policy UID")
        require(bool(re.fullmatch(r"[A-Za-z0-9._-]{1,128}", str(im.get("resourceVersion", "")))), "invalid resourceVersion")
        require(isinstance(item.get("spec"), dict), "missing policy spec")
    return items


def verify_positive_baseline(probe, policy_absent, routes, attempts=20, delay=3, sleep=time.sleep, emit=print):
    """Bounded startup settling; never treats a failed positive HTTP probe as healthy.

    probe(client, url) returns (success: bool, failure_summary: str).
    Each attempt must pass both clients through BOTH service and Pod IP routes.
    """
    require(set(routes) == {"service", "pod_ip"}, "baseline requires service and Pod IP targets")
    require(isinstance(attempts, int) and 1 <= attempts <= 30, "invalid baseline retry limit")
    require(delay >= 0, "invalid baseline delay")
    last_failures = []
    for round_no in range(1, attempts + 1):
        require(policy_absent(), "baseline has unexpected NetworkPolicy")
        failed = []
        for target in ("service", "pod_ip"):
            url = routes[target]
            for client in ("trusted-client", "edge-client"):
                success, detail = probe(client, url)
                require(isinstance(success, bool), "baseline probe returned non-boolean outcome")
                if not success:
                    failed.append(f"{target}/{client}: {str(detail)[:180]}")
        if not failed:
            emit(f"CI_BASELINE: 4/4 HTTP probes passed in round {round_no}/{attempts}")
            return
        last_failures = failed
        if round_no == 1 or round_no % 5 == 0 or round_no == attempts:
            emit(f"CI_BASELINE_WAIT: round {round_no}/{attempts} failures={len(failed)} " + "; ".join(failed))
        if round_no < attempts:
            sleep(delay)
    raise GateFailure("positive baseline failed after bounded readiness checks: " + "; ".join(last_failures))


class Live:
    def __init__(self, name, kubeconfig, repo, livegate, driftgate):
        require(bool(NAME_RE.fullmatch(name)), "refusing non-ephemeral cluster")
        self.name, self.context = name, "kind-" + name
        self.kubeconfig = pathlib.Path(kubeconfig).resolve(strict=True)
        self.repo = pathlib.Path(repo).resolve(strict=True)
        self.livegate = pathlib.Path(livegate).resolve(strict=True)
        self.driftgate = pathlib.Path(driftgate).resolve(strict=True)
        self.uid = None
        self.snapshot_dir = tempfile.TemporaryDirectory(prefix="firedoor-v09-ci-observations-")

    def close(self):
        self.snapshot_dir.cleanup()

    def kubectl(self, *args, timeout=120, ok=True):
        p = command(["kubectl", "--kubeconfig", str(self.kubeconfig), "--context", self.context, *args], timeout)
        if ok:
            require(p.returncode == 0, "kubectl failed (" + str(p.returncode) + "): " + p.stderr[-500:])
        return p

    def policy(self):
        raw = self.kubectl("get", "--raw", "/apis/networking.k8s.io/v1/namespaces/" + NS + "/networkpolicies").stdout
        try:
            doc = json.loads(raw)
        except (ValueError, TypeError) as exc:
            raise GateFailure("invalid live Kubernetes JSON") from exc
        items = parse_list(doc)
        return items[0] if items else None

    def policy_uid(self, item):
        return item["metadata"]["uid"]

    def run_go(self, executable, payload, expected_exit, expected_verdict, label):
        file = pathlib.Path(self.snapshot_dir.name) / (label + ".json")
        file.write_text(json.dumps(payload, separators=(",", ":")), encoding="utf-8")
        p = command([str(executable), "--input", str(file)])
        require(p.returncode == expected_exit, label + " unexpected Go exit " + str(p.returncode) + ": " + p.stderr[-350:] + p.stdout[-350:])
        try:
            result = json.loads(p.stdout)
        except ValueError as exc:
            raise GateFailure(label + " invalid Go result") from exc
        require(result.get("verdict") == expected_verdict, label + " wrong Go verdict")
        return result

    def drift(self, item, phase, expected_exit, expected):
        present = item is not None
        meta = item["metadata"] if present else {}
        doc = {
            "schema_version": "firedoor.v09.drift.snapshot.v1", "source": "guarded_kind_calico_api",
            "run_id": self.name, "cluster": self.name, "context": self.context,
            "namespace": NS, "policy_name": POLICY,
            "observation_state": "found" if present else "not_found",
            "policy_uid": meta.get("uid", ""), "resource_version": meta.get("resourceVersion", ""),
            "spec": item["spec"] if present else None,
        }
        self.run_go(self.driftgate, doc, expected_exit, expected, "drift-" + phase)
        print(f"CI_DRIFT: phase={phase} verdict={expected} exit={expected_exit}", flush=True)

    def exec_client(self, client, url):
        p = self.kubectl("-n", NS, "exec", client, "--", "wget", "-q", "-T", "3", "-O", "-", url, timeout=18, ok=False)
        if p.returncode == 0 and MARKER not in p.stdout:
            raise GateFailure("successful client response without expected marker")
        return p.returncode == 0

    def baseline_probe(self, client, url):
        # Retain stderr during baseline startup diagnostics; the protected probes
        # continue to use exec_client and unchanged policy-verdict logic.
        p = self.kubectl("-n", NS, "exec", client, "--", "wget", "-q", "-T", "3", "-O", "-", url, timeout=18, ok=False)
        if p.returncode == 0 and MARKER not in p.stdout:
            raise GateFailure("successful baseline response without expected marker")
        if p.returncode != 0:
            stderr = " ".join(p.stderr.split())[-160:] or "no wget stderr"
            return False, f"kubectl/wget exit {p.returncode}: {stderr}"
        return True, "marker matched"

    def baseline_diagnostics(self):
        # Only describe resources inside the generated kind cluster. No writes.
        for label, arguments in (
            ("service", ("-n", NS, "get", "service", "data", "-o", "wide")),
            ("endpoints", ("-n", NS, "get", "endpointslice", "-l", "kubernetes.io/service-name=data", "-o", "wide")),
            ("kube-system", ("-n", "kube-system", "get", "pods", "-o", "wide")),
            ("dns", ("-n", NS, "exec", "trusted-client", "--", "nslookup", "data." + NS + ".svc.cluster.local")),
        ):
            try:
                outcome = self.kubectl(*arguments, timeout=25, ok=False)
                details = " ".join((outcome.stdout + " " + outcome.stderr).split())[-900:]
                print(f"CI_BASELINE_DIAG: {label} exit={outcome.returncode} {details}", flush=True)
            except (OSError, subprocess.TimeoutExpired) as exc:
                print(f"CI_BASELINE_DIAG: {label} unavailable: {str(exc)[:160]}", flush=True)

    def exec_healthy(self, client):
        return self.kubectl("-n", NS, "exec", client, "--", "sh", "-c", "echo FIREDOOR_EXEC_OK", timeout=15).stdout.strip() == "FIREDOOR_EXEC_OK"

    def cni_ready(self):
        result = json.loads(self.kubectl("-n", "kube-system", "get", "ds", "calico-node", "-o", "json").stdout)
        st = result.get("status", {})
        return st.get("desiredNumberScheduled") == 2 and st.get("numberReady") == 2

    def endpoints(self):
        pod = json.loads(self.kubectl("-n", NS, "get", "pod", "data-server", "-o", "json").stdout)
        ip = pod.get("status", {}).get("podIP", "")
        try:
            ipaddress.ip_address(ip)
        except ValueError as exc:
            raise GateFailure("invalid data-server Pod IP") from exc
        return {"service": f"http://data.{NS}.svc.cluster.local:8080/", "pod_ip": f"http://{ip}:8080/"}

    def wait_route(self, url, allowed, label):
        for _ in range(12):
            require(self.exec_client("trusted-client", url), "trusted positive control failed for " + label)
            require(self.exec_healthy("edge-client"), "edge client process unavailable")
            observed = self.exec_client("edge-client", url)
            require(self.exec_client("trusted-client", url), "trusted positive control failed after edge probe")
            if observed == allowed:
                return "reachable" if observed else "unreachable"
            time.sleep(2)
        raise GateFailure(f"{label}: expected {'reachable' if allowed else 'blocked'} edge traffic")

    def live(self, item, phase, allowed):
        require(item is not None and self.policy_uid(item) == self.uid, "policy replaced during test")
        expected_spec = WEAKENED if phase == "weakened" else APPROVED
        require(item["spec"] == expected_spec, "phase policy spec not verified against exact fixture")
        require(self.cni_ready(), "Calico readiness lost")
        for target, url in self.endpoints().items():
            observed = self.wait_route(url, allowed, phase + ":" + target)
            doc = {
                "schema_version": "firedoor.v09.regression.live.v1", "source": "guarded_kind_calico_probe",
                "run_id": self.name, "cluster": self.name, "context": self.context,
                "namespace": NS, "policy_name": POLICY, "policy_uid": self.uid,
                "phase": phase, "policy_mode": "all_namespace_pods" if phase == "weakened" else "trusted_only",
                "route_id": "edge-to-data", "target": target, "transport": "HTTP/TCP", "port": 8080,
                "observed": observed, "trusted_healthy": True, "edge_exec_healthy": True,
                "cni_ready": True, "policy_verified": True,
            }
            expected, exit_code = ("REGRESSION", 10) if allowed else ("MATCH", 0)
            self.run_go(self.livegate, doc, exit_code, expected, "live-" + phase + "-" + target)
            print(f"CI_LIVE: phase={phase} target={target} verdict={expected} exit={exit_code}", flush=True)

    def required_healthy_gate(self, item, label):
        # This is the required CI pass contract. A weak/missing policy must NOT pass.
        self.drift(item, label, 0, "MATCH")
        self.live(item, "restored" if label == "restored" else "protected", False)

    def run(self):
        try:
            nsfile = self.repo / "infrastructure/kind/v09c1-positive-baseline.yaml"
            protected = self.repo / "infrastructure/kind/v09c2-trusted-ingress-policy.yaml"
            weakened = self.repo / "infrastructure/kind/v09d1-weakened-ingress-policy.yaml"
            require(self.kubectl("config", "current-context").stdout.strip() == self.context, "wrong kind context")
            self.kubectl("apply", "-f", str(nsfile))
            for pod in ("data-server", "edge-client", "trusted-client"):
                self.kubectl("-n", NS, "wait", "--for=condition=Ready", "pod/" + pod, "--timeout=180s", timeout=205)
            require(self.policy() is None, "baseline has unexpected policy")
            try:
                verify_positive_baseline(self.baseline_probe, lambda: self.policy() is None,
                                         self.endpoints(), attempts=20, delay=3)
            except GateFailure:
                self.baseline_diagnostics()
                raise

            self.kubectl("apply", "-f", str(protected))
            item = self.policy()
            require(item is not None, "approved policy not found")
            self.uid = self.policy_uid(item)
            self.required_healthy_gate(item, "protected")

            self.kubectl("apply", "-f", str(weakened))
            item = self.policy()
            require(item is not None and self.policy_uid(item) == self.uid, "policy UID changed on weakening")
            self.drift(item, "weakened", 10, "DRIFT")
            self.live(item, "weakened", True)
            # Deliberately run the healthy gate against a real weakened state.
            # The gate must refuse to pass: this is our actual failure test.
            try:
                self.required_healthy_gate(item, "weakened-as-protected")
            except GateFailure as exc:
                require("unexpected Go exit 10" in str(exc), "weak policy rejected for the wrong reason")
                print("CI_NEGATIVE: real weakened policy correctly FAILED the required health gate", flush=True)
            else:
                raise GateFailure("unsafe: weakened policy PASSED required health gate")

            self.kubectl("apply", "-f", str(protected))
            item = self.policy()
            require(item is not None and self.policy_uid(item) == self.uid, "policy UID changed on restoration")
            self.required_healthy_gate(item, "restored")

            self.kubectl("-n", NS, "delete", "networkpolicy", POLICY, "--wait=true", "--timeout=60s")
            require(self.policy() is None, "policy still present after deletion")
            self.drift(None, "missing", 10, "DRIFT")
            for target, url in self.endpoints().items():
                self.wait_route(url, True, "missing:" + target)
            print("PASS: Firedoor 9F1 CI live MATCH/DRIFT/MATCH/missing DRIFT, fail-gate test, and recovery", flush=True)
        finally:
            self.close()


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--self-test", action="store_true")
    p.add_argument("--cluster")
    p.add_argument("--kubeconfig")
    p.add_argument("--repo")
    p.add_argument("--livegate")
    p.add_argument("--driftgate")
    args = p.parse_args()
    try:
        if args.self_test:
            require(not any((args.cluster, args.kubeconfig, args.repo, args.livegate, args.driftgate)), "self-test cannot take live arguments")
            self_test()
        else:
            require(all((args.cluster, args.kubeconfig, args.repo, args.livegate, args.driftgate)), "live run requires all scoped arguments")
            client = Live(args.cluster, args.kubeconfig, args.repo, args.livegate, args.driftgate)
            client.run()
    except (GateFailure, OSError, subprocess.TimeoutExpired, ValueError, AssertionError) as exc:
        print("FAIL: Firedoor CI live gate: " + str(exc), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
