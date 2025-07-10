$COMMAND = $args[0]

$NAME = "mute-service"
$OWNER = "byuoitav"
$PKG = "github.com/$OWNER/$NAME"
$DOCKER_URL = "ghcr.io"
$DOCKER_PKG = "$DOCKER_URL/$OWNER/$NAME"

Write-Output "PKG: $PKG"
Write-Output "DOCKER_PKG: $DOCKER_PKG"

$PRD_TAG_REGEX = "v[0-9]+\.[0-9]+\.[0-9]+"
$DEV_TAG_REGEX = "v[0-9]+\.[0-9]+\.[0-9]+-.+"


$COMMIT_HASH = Invoke-Expression "git rev-parse --short HEAD"
$TAG = Invoke-Expression "git rev-parse --short HEAD"
try {
    $NEW_TAG = Invoke-Expression "git describe --exact-match --tags HEAD"
    if ($NEW_TAG.Length -gt 0) {
        $TAG = $NEW_TAG
        Write-Output "The repo contains a tag: $TAG"
    }
}
catch {
    Write-Output "The repo does not contain a tag"
}

Write-Output "The TAG is: $TAG"

# go stuff
$PKG_LIST = Invoke-Expression "go list $PKG/..."
Write-Output "PKG_LIST: $PKG_LIST"


function All {
    Write-Output "All"
    Cleanup
    Build
}


function Deps {
    Write-Output "Downloading backend dependencies..."
    Invoke-Expression "go mod download"
}

function Build {
    Write-Output "Build"
    Deps

    New-Item -Path dist -ItemType Directory -Force

    # Store initial environment variables
    $ORIGINAL_CGO_ENABLED = $env:CGO_ENABLED
    $ORIGINAL_GOOS = $env:GOOS
    $ORIGINAL_GOARCH = $env:GOARCH

    Write-Output ""
    Write-Output "Building mute service for linux-amd64..."
    $env:CGO_ENABLED = "0"
    $env:GOOS = "linux"
    $env:GOARCH = "amd64"
    Invoke-Expression "go build -v -o ./dist/${NAME}-linux-amd64"

    Write-Output ""
    Write-Output "Building mute service for linux-arm..."
    $env:CGO_ENABLED = "0"
    $env:GOOS = "linux"
    $env:GOARCH = "arm"
    Invoke-Expression "go build -v -o ./dist/${NAME}-linux-arm"

    # Restore original environment variables
    $env:CGO_ENABLED = $ORIGINAL_CGO_ENABLED
    $env:GOOS = $ORIGINAL_GOOS
    $env:GOARCH = $ORIGINAL_GOARCH

    Write-Output ""
    Write-Output "Build output is located in ./dist/."
}

function Cleanup {
    Write-Output "Clean"
    Invoke-Expression "go clean"
    if (Test-Path -Path "dist") {
        Remove-Item dist -Recurse
        Write-Output "Recursively deleted dist/"
    }
    else {
        Write-Output "No dist directory to delete"
    }
}

function DockerFunc {
    Write-Output "Function Docker      Commit Hash: $COMMIT_HASH     Tag: $TAG"
    
    if ($COMMIT_HASH -eq $TAG) {
        Write-Output "Building dev container with tag $COMMIT_HASH"

        Write-Output "Building container $DOCKER_PKG/${NAME}-dev:$COMMIT_HASH"
        Invoke-Expression "docker build -f dockerfile --build-arg NAME=${NAME}-linux-arm -t $DOCKER_PKG/${NAME}-dev:$COMMIT_HASH dist"
    }
    elseif ($TAG -match $DEV_TAG_REGEX) {
        Write-Output "Building dev container with tag $TAG"

        Write-Output "Building container $DOCKER_PKG/${NAME}-dev:$TAG"
        Invoke-Expression "docker build -f dockerfile --build-arg NAME=${NAME}-linux-arm -t $DOCKER_PKG/${NAME}-dev:$TAG dist"
    }
    elseif ($TAG -match $PRD_TAG_REGEX) {
        Write-Output "Building prd container with tag $TAG"

        Write-Output "Building container $DOCKER_PKG/${NAME}:$TAG"
        Invoke-Expression "docker build -f dockerfile --build-arg NAME=${NAME}-linux-arm -t $DOCKER_PKG/${NAME}:$TAG dist"
    }
    else {
        Write-Output "Docker function quit unexpectedly. Commit Hash: $COMMIT_HASH     Tag: $TAG"
    }
}

function Deploy {
    Write-Output "Deploy      Commit Hash: $COMMIT_HASH     Tag: $TAG"

    Write-Output "Logging into Github Container Registry"
    Invoke-Expression "docker login $DOCKER_URL -u $Env:DOCKER_USERNAME -p $Env:DOCKER_PASSWORD"

    if ($COMMIT_HASH -eq $TAG) {
        Write-Output "Pushing dev container with tag $COMMIT_HASH"

        Write-Output "Pushing container $DOCKER_PKG/${NAME}-dev:$COMMIT_HASH"
        Invoke-Expression "docker push $DOCKER_PKG/${NAME}-dev:$COMMIT_HASH"
    }
    elseif ($TAG -match $DEV_TAG_REGEX) {
        Write-Output "Pushing dev container with tag $TAG"

        Write-Output "Pushing container $DOCKER_PKG/${NAME}-dev:$TAG"
        Invoke-Expression "docker push $DOCKER_PKG/${NAME}-dev:$TAG"
    }
    elseif ($TAG -match $PRD_TAG_REGEX) {
        Write-Output "Pushing prd container with tag $TAG"

        Write-Output "Pushing container $DOCKER_PKG/${NAME}:$TAG"
        Invoke-Expression "docker push $DOCKER_PKG/${NAME}:$TAG"
    }
    else {
        Write-Output "Deploy function quit unexpectedly. Commit Hash: $COMMIT_HASH     Tag: $TAG"
    }
}

if ($COMMAND -eq "All") {
    All
}
elseif ($COMMAND -eq "Deps") {
    Deps
}
elseif ($COMMAND -eq "Build") {
    Build
}
elseif ($COMMAND -eq "Clean") {
    Cleanup
}
elseif ($COMMAND -eq "Docker") {
    Cleanup
    Build
    DockerFunc
}
elseif ($COMMAND -eq "DockerOnly") {
    DockerFunc
}
elseif ($COMMAND -eq "Deploy") {
    Cleanup
    Build
    DockerFunc
    Deploy
}
elseif ($COMMAND -eq "DeployOnly") {
    Deploy
}
else {
    Write-Output "Please include a valid command parameter"
    Write-Output "Available commands: All, Deps, Build, Clean, Docker, DockerOnly, Deploy, DeployOnly"
}
