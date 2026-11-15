# Security

No account system, trade execution or server-side watchlist storage. All market APIs are public; place access controls and TLS at a reverse proxy if needed. Private provider keys belong only in server environment variables, never localStorage or git. The server does not log query values, request bodies or upstream errors containing URLs. Reverse proxy logs are deployer-controlled.

Upstream hosts are fixed HTTPS allowlists, redirects disabled, bodies bounded, asset IDs validated, per-IP API limits and global upstream concurrency/cooldowns enforced. A bounded shared memory cache stores only public market data with no user association. Static delivery does not expose source files. CSP permits inline styles needed by ECharts, but script code must be same-origin.

Report suspected vulnerabilities privately to the maintainer without publishing keys or user data in public issues.
