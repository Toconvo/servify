Self-Hosted Runner Setup (GitHub Actions)

1) Provision a host
- OS: Linux x64 recommended (2 vCPU, 4 GB RAM+)
- Install: git, curl, Go toolchain, Docker (optional for Docker workflows)

2) Register a self-hosted runner
- Repo: Settings → Actions → Runners → New self-hosted runner
- Choose Linux → x64 → follow commands; example:
```
mkdir -p ~/actions-runner && cd ~/actions-runner
curl -o actions-runner.tar.gz -L https://github.com/actions/runner/releases/download/v2.319.1/actions-runner-linux-x64-2.319.1.tar.gz
tar xzf actions-runner.tar.gz
./config.sh --url https://github.com/<org>/<repo> --token <TOKEN>
./run.sh
```
- Optional: install as a service:
```
sudo ./svc.sh install
sudo ./svc.sh start
```

3) Labels
- Default label: `self-hosted`
- If you add custom labels (e.g. `servify`), edit `.github/workflows/ci.yml` `runs-on` accordingly.

4) Required tools on runner
- Go (reads version from go.mod via actions/setup-go)
- Permit network to fetch modules
- If building Docker images, ensure Docker is installed and runner user is in `docker` group

5) First run
- Push to `main` or open PR, workflow `.github/workflows/ci.yml` triggers on self-hosted runner

Troubleshooting
- Ensure runner is online (green) in repo settings
- Check runner logs under `~/actions-runner/_diag`
- If module download is blocked, pre-populate `~/go/pkg/mod` or add a proxy

