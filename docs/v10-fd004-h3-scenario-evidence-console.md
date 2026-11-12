# H3 — FD-004 historical evidence in the console

## What this adds

From **Scenarios → Lateral Movement (FD-004) → View contract**, a user can select
`Firedoor-H2-ScenarioResult.json` generated locally by H2. The browser displays
one supporting historical Edge → Data observation, three positive controls, and
four unverified/not-met requirements. The scenario remains **Not tested**.

The browser accepts only the exact H2 `firedoor.v10.scenario-result.v1`
historical result contract. It rejects extra fields, unsupported outcomes,
reordered evidence, altered requirements, and positive claims about current
enforcement, certification, cluster scope, or original receipt authentication.

### Boundaries

- No API route changes, automatic fetching, file persistence or uploads.
- User selection is local to the browser; closing the dialog discards the data.
- H3 validates the display schema, **not** the authenticity of the original
  F5 private receipt or current Kubernetes state.
- A SHA-256 file check at export time only establishes that the earlier file
  had the expected bytes. The browser does not independently establish provenance.
- The **Open Live Proof** action goes to the separate historical G1 projection
  view. It never executes a network request or automatically loads evidence.

### Local review

1. Start the already-existing local console API and React dev server.
2. Open **Scenarios**, then **Lateral Movement / FD-004**.
3. Select the H2 result in Downloads via the local file picker.
4. Review the saved observation and four unmet/unverified requirements.
5. Confirm other scenarios still show **Not tested**, and clearing/closing
   the panel does not persist the selected file.

H3 does not implement the FD-004 scenario, assess its mapped controls, or
certify the cluster. Future H4/H5 work must preserve these boundaries.
