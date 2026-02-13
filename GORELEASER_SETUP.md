# GoReleaser & Homebrew Setup Guide

This guide walks you through setting up GoReleaser and the Homebrew tap for seekarr.

## Prerequisites

- GoReleaser configuration is already in place (`.goreleaser.yaml`)
- GitHub Actions workflow is configured (`.github/workflows/release.yml`)
- Version flag support added to the CLI

## Step 1: Create the Homebrew Tap Repository

1. Go to GitHub and create a new repository named `homebrew-seekarr`
   - **Important:** The repository MUST be named `homebrew-seekarr` (starting with `homebrew-`)
   - It can be public or private (public is recommended for easy access)
   - Initialize it with a README if you want, but it's not required

2. The repository URL will be: `https://github.com/yuritomanek/homebrew-seekarr`

## Step 2: Create a GitHub Personal Access Token

GoReleaser needs permission to push the Homebrew formula to your tap repository.

1. Go to GitHub Settings → Developer settings → Personal access tokens → Tokens (classic)
   - Or visit: https://github.com/settings/tokens

2. Click "Generate new token (classic)"

3. Configure the token:
   - **Note:** "GoReleaser Homebrew Tap Access"
   - **Expiration:** No expiration (or your preference)
   - **Scopes:** Select `repo` (this includes all sub-scopes)

4. Click "Generate token"

5. **IMPORTANT:** Copy the token immediately - you won't see it again!

## Step 3: Add the Token to Your Repository Secrets

1. Go to your seekarr repository on GitHub

2. Navigate to Settings → Secrets and variables → Actions

3. Click "New repository secret"

4. Add the secret:
   - **Name:** `HOMEBREW_TAP_TOKEN`
   - **Value:** Paste the token you copied in Step 2

5. Click "Add secret"

## Step 4: Test GoReleaser Locally (Optional)

Before creating a release, you can test GoReleaser locally:

```bash
# Install goreleaser
brew install goreleaser

# Test the configuration (dry run - doesn't publish)
goreleaser release --snapshot --clean --skip=publish

# This will create builds in ./dist/ folder
ls -la dist/
```

## Step 5: Create Your First Release

1. Make sure all your changes are committed and pushed:

```bash
git add .
git commit -m "Setup GoReleaser and Homebrew tap"
git push origin main
```

2. Create and push a new version tag:

```bash
# Create a new tag (use semantic versioning)
git tag v0.2.0

# Push the tag to GitHub
git push origin v0.2.0
```

3. GitHub Actions will automatically:
   - Run tests
   - Build binaries for all platforms (macOS Intel, macOS ARM, Linux x86_64, Linux ARM64)
   - Create a GitHub release with all artifacts
   - Calculate SHA256 checksums
   - Generate release notes from commits
   - **Automatically create/update the Homebrew formula** in your tap repository

## Step 6: Verify the Release

1. Check the GitHub Actions run:
   - Go to your repository → Actions tab
   - You should see a "Release" workflow running

2. Once complete, check:
   - **GitHub Release:** Should have all platform binaries and checksums
   - **Homebrew Tap:** Check `https://github.com/yuritomanek/homebrew-seekarr`
     - Should have a `Formula/seekarr.rb` file
     - The formula should reference your v0.2.0 release

## Step 7: Test Installing via Homebrew

```bash
# Add your tap
brew tap yuritomanek/seekarr

# Install seekarr
brew install seekarr

# Verify it works
seekarr --version

# Check where the example config was installed
ls -la /opt/homebrew/etc/seekarr/  # Apple Silicon
ls -la /usr/local/etc/seekarr/     # Intel Mac
```

## Step 8: Update the README

Add Homebrew installation instructions to your README.md:

```markdown
### Homebrew (macOS/Linux)

```bash
brew tap yuritomanek/seekarr
brew install seekarr
```

Copy and edit the example config:
```bash
mkdir -p ~/.config/seekarr
cp /opt/homebrew/etc/seekarr/config.example.yaml ~/.config/seekarr/config.yaml
# Edit with your API keys
```
```

## Future Releases

Every time you want to create a new release:

```bash
# 1. Commit your changes
git add .
git commit -m "Your changes"
git push

# 2. Create a new tag
git tag v0.3.0
git push origin v0.3.0
```

GoReleaser will automatically:
- Build for all platforms
- Create the GitHub release
- Update the Homebrew formula with the new version
- Users can update with: `brew upgrade seekarr`

## Troubleshooting

### Error: "HOMEBREW_TAP_TOKEN not found"

Make sure you added the secret in Step 3. The secret name must be exactly `HOMEBREW_TAP_TOKEN`.

### Error: "Permission denied" when pushing to tap

Check that your personal access token has the `repo` scope enabled.

### Formula not updating

Check the GitHub Actions logs. The most common issue is the token not having the right permissions.

### Testing the formula locally

You can test changes to `.goreleaser.yaml` without creating a release:

```bash
goreleaser release --snapshot --clean --skip=publish
```

## Additional Features

GoReleaser can do much more:

- **Docker images:** Add Docker configuration to publish to Docker Hub/GHCR
- **Scoop/Chocolatey:** Add Windows package managers
- **AUR:** Arch Linux packages
- **Snapcraft:** Ubuntu Snap packages
- **Linux packages:** .deb and .rpm files

See: https://goreleaser.com/customization/
