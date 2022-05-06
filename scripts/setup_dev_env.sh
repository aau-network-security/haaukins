#!/bin/bash
################################################################################################################
# This script prepares an development environment on macOS and Debian based computers
# Tested on Debian and macOS environment however check the script before using it
# USE IT WITH YOUR OWN RISKS !!!
####################################################################################################################


HAAUKINS_REPO="git@github.com:aau-network-security/haaukins.git"
HAAUKINS_STORE_REPO="git@github.com:aau-network-security/haaukins-store.git"
HAAUKINS_WEBCLIENT_REPO="git@github.com:aau-network-security/haaukins-webclient.git"
HAAUKINS_EXERCISES_REPO="git@github.com:aau-network-security/haaukins-exercises.git"
VIRTUAL_DEV_ENV="git@github.com:aau-network-security/sec0x.git"

LEAST_GO_VERSION=1.15
VBOX_VERSION=6.1.34
GO_VERSION=1.18.1

PROGRAMS=(go vagrant packer)
REPOS=($HAAUKINS_REPO $HAAUKINS_STORE_REPO $HAAUKINS_EXERCISES_REPO $HAAUKINS_WEBCLIENT_REPO)

PROJECT_DIR=$HOME/Documents/project

mkdir -p $PROJECT_DIR

cd $PROJECT_DIR

 # clone repositories
 for i in ${REPOS[@]}; do
     echo "Cloning repository $i"
     git clone $i
 done


CHECK_GOLANG_VERSION() {
  v=`go version | { read _ _ v _; echo ${v#go}; }`
  if (( $(echo "$v >= $LEAST_GO_VERSION" |bc -l) )); then
      echo " Your Go version $v is suitable for Haaukins"
  else
      echo "Your Go version $v does not support Haaukins to run."
      echo "Please consider your Go environment before starting."
      echo "Exiting..."
      exit 1
  fi
}


# Install golang
# No need to call for macOS
# Since macOS uses Vagrant environment
INSTALL_GOLANG_MACOS() {
    which -s go
    if [[ $? != 0 ]] ; then
       # Install golang
        echo "Installing GoLang..."
        wget https://go.dev/dl/go$GO_VERSION.darwin-amd64.pkg
        sudo installer -pkg go$GO_VERSION.darwin-amd64.pkg -target ~/Applications/
    else
      echo "Go is already installed !!"
      CHECK_GOLANG_VERSION
    fi
}


# install homebrew
INSTALL_HOMEBREW() {

    which -s brew
    if [[ $? != 0 ]] ; then
       # Install Homebrew
        echo "Installing homebrew..."
        "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/master/install)"
    else
        echo "Homebrew is already installed, updating brew."
        brew update
    fi
}

# install vagrant
INSTALL_VAGRANT_MACOS () {
    which -s vagrant
    if [[ $? != 0 ]] ; then
       # Install Vagrant
       echo "Installing vagrant through brew"
       brew install vagrant
    else
        echo "vagrant is already installed"
    fi

}


# install packer
INSTALL_PACKER_MACOS() {
    which -s packer
    if [[ $? != 0 ]] ; then
       # Install Packer
        echo "Installing packer through brew"
        brew tap hashicorp/tap
        brew install hashicorp/tap/packer
    else
        echo "Packer is already installed"
      #  brew upgrade hashicorp/tap/packer
    fi
}

BUILD_VM_ENV() {
    echo "Building VM environment"
    cd $PROJECT_DIR/sec0x/hkn-base
    ./build.sh
    cd $PROJECT_DIR/sec0x
    # make sure plugins are installed
    echo "Installing plugin: vagrant-disksize"
    vagrant plugin install vagrant-disksize
    echo "Installing plugin: vagrant-vbguest"
    vagrant plugin install vagrant-env
    echo "Starting machine with vagrant up"
    vagrant up
}

INSTALL_GAWK () {
  which -s gawk
    if [[ $? != 0 ]] ; then
       echo "gawk is already installed"
    else
        # Install Gawk
        echo "Installing gawk through brew"
        brew install gawk
    fi
}


INSTALL_VIRTUALBOX_DEBIAN() {

    if which vboxmanage >/dev/null; then
        echo "vbox is already installed"
    else
        echo "Installing vboxmanage ... "
        sudo apt update 
        curl https://download.virtualbox.org/virtualbox/$VBOX_VERSION/Oracle_VM_VirtualBox_Extension_Pack-$VBOX_VERSION.vbox-extpack --output Oracle_VM_VirtualBox_Extension_Pack-$VBOX_VERSION.vbox-extpack
        sudo VBoxManage extpack install Oracle_VM_VirtualBox_Extension_Pack-$VBOX_VERSION.vbox-extpack
        sudo apt install virtualbox -y
    fi

}

# install docker engine 
INSTALL_DOCKER() {
    if which docker > /dev/null; then
        echo "Docker is already installed !"
    else
        echo "Installing docker engine ... "
        curl -fsSL https://get.docker.com -o get-docker.sh
        sh ./get-docker.sh
        sudo usermod -aG docker $USER
        sudo apt install docker-compose -y 
        rm ./get-docker.sh 
    fi 
}


# install packer
INSTALL_PACKER_DEBIAN() {
    if which packer >/dev/null; then
        echo "Packer is already installed"
    else
        # Install Packer
        echo "Installing packer through apt install"
        curl -fsSL https://apt.releases.hashicorp.com/gpg | sudo apt-key add -
        sudo apt-add-repository "deb [arch=amd64] https://apt.releases.hashicorp.com $(lsb_release -cs) main"
        sudo apt-get update && sudo apt-get install -y packer
    fi
}

INSTALL_VAGRANT_DEBIAN(){
    if which vagrant >/dev/null; then
       echo "vagrant is already installed"
    else
     # Install Vagrant
       echo "Installing vagrant through apt install"
       curl -fsSL https://apt.releases.hashicorp.com/gpg | sudo apt-key add -
       sudo apt-add-repository "deb [arch=amd64] https://apt.releases.hashicorp.com $(lsb_release -cs) main"
       sudo apt-get update -y && sudo apt-get install vagrant -y
    fi
}

# install golang 
INSTALL_GOLANG_DEBIAN() {
    if which go >/dev/null; then
        echo "Go is already installed !!"
        CHECK_GOLANG_VERSION
    else
       # Install golang
        echo "Installing GoLang..."
        wget https://go.dev/dl/go$GO_VERSION.linux-amd64.tar.gz
        sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go$GO_VERSION.linux-amd64.tar.gz
        echo "export PATH=$PATH:/usr/local/go/bin" >> ~/.bashrc
    fi
}




if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    INSTALL_GOLANG_DEBIAN
    INSTALL_PACKER_DEBIAN
    INSTALL_VIRTUALBOX_DEBIAN
    INSTALL_VAGRANT_DEBIAN
    INSTALL_DOCKER
elif [[ "$OSTYPE" == "darwin"* ]]; then
    INSTALL_HOMEBREW
    INSTALL_GAWK
    INSTALL_PACKER_MACOS
    INSTALL_VAGRANT_MACOS
 #   BUILD_VM_ENV # this is an optional choice to enable 
else 
    echo "Setup script does not support your environment yes"
fi 



