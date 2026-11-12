# G3C — Public README showcase of historical network observations

The screenshots in `docs/screenshots/live-proof/` were captured by G3B from the actual React console, with an operator-selected **sanitized G1 historical projection**, not by querying the Kubernetes API from the browser.

Image SHA-256 fingerprints:

- `02-loaded-proof-overview.png`: `B8489C5D61FD1F8B0B1E70415308747223CDDEBCD64FCD767CE57B179382A641`
- `03-allowed-app-data.png`: `54A6D3946DF6E988326146539ED94D2FA9C7330CD95480981DA2EE4D1474780C`
- `04-blocked-data-app.png`: `00ECAEEF49D88A6553BF33B38C6FD727BBA93B8EBFE68E3AB485348BE46F510E`

Capture reference: **2026-11-12 03:27:30 UTC**, five HTTP observations, three expected successes and two bounded connection timeouts. The original private F5 receipt and Terraform local state are **not** distributed with the repository. No evidence projection is automatically loaded.

### Editorial selection

- **02** provides the context and honest “Current live check: NO” card above the fold; use it as the README's primary screenshot.
- **03** illustrates allowed App → Data traffic.
- **04** illustrates blocked Data → App traffic; the observation text is timeout/no HTTP response, not independent absolute policy attribution.
- **01** is a useful empty-state regression capture, and **05** a limitations review, but neither is needed above the fold.
- **06** reveals a horizontally overflowing mobile navigation strip. Do **not** publish it as evidence of a polished mobile experience until the global mobile navigation is fixed and the capture is repeated.

### Reproducibility / safety

- Readers can run the default read-only console without Kubernetes or credentials.
- To explore the historical route cards they need a **legitimate, locally generated** G1 projection from their own approved F5 receipt. No real projection or raw receipt is checked in.
- Screenshot capture requires a local dev server and explicit sanitized proof file; it neither deploys infrastructure nor validates current cluster state.
- Desktop captures are documentation only; do not represent current production or disaster-recovery evidence.
