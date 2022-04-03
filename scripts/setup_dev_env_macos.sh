#!/bin/bash
# This file is particularly written for macOS operating system
# Tested on Intel based macOS

# todo: check Golang installation  and its version
# todo: check Vagrant installation
# todo: check Packer
# todo: get sec0x from aau-network-security and install it
# todo: install gawk to use that command on macos truly: brew install gawk

HAAUKINS_REPO="git@github.com:aau-network-security/haaukins.git"
HAAUKINS_STORE_REPO="git@github.com:aau-network-security/haaukins-store.git"
HAAUKINS_WEBCLIENT_REPO="git@github.com:aau-network-security/haaukins-webclient.git"
HAAUKINS_EXERCISES_REPO="git@github.com:aau-network-security/haaukins-exercises.git"
VIRTUAL_DEV_ENV="git@github.com:aau-network-security/sec0x.git"

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

# clone repositories
for i in ${PROGRAMS[@]}; do
    echo "Checking if $i exits on system or not"
    if ! $i version  &> /dev/null
    then
        echo "$i could not be found"
        exit
    else
        echo "$i is exits, version information: $($i version) "
        if [ $i == "go" ]
        then
             v=`go version | { read _ _ v _; echo ${v#go}; }`
             required_version="1.17"
             result=$(awk -vx=$v -vy=$required_version 'BEGIN{ print x<y?1:0}')
            if [ "$result" -eq 1 ]
            then
                 echo "Your version is outdated please upgrade it to 1.16 or later version"
            fi
        fi
    fi
done



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
INSTALL_VAGRANT () {
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
INSTALL_PACKER() {
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
    cd $PROJECT_DIR/sec0x/hkn-base
    ./build.sh
    cd $PROJECT_DIR/sec0x
    # make sure plugins are installed
    vagrant plugin install vagrant-disksize
    vagrant plugin install vagrant-env
    vagrant up
}

# call functions to check

INSTALL_HOMEBREW
INSTALL_PACKER
INSTALL_VAGRANT







