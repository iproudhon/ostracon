# -*- mode: ruby -*-
# vi: set ft=ruby :

Vagrant.configure("2") do |config|
  config.vm.box = "ubuntu/focal64"

  config.vm.provider "virtualbox" do |v|
    v.memory = 4096
    v.cpus = 2
  end

  config.vm.provision "shell", inline: <<-SHELL
    apt-get update

    # install base requirements
    apt-get install -y --no-install-recommends wget curl jq zip \
            make build-essential shellcheck bsdmainutils psmisc
    apt-get install -y language-pack-en

    # install docker
    apt-get install -y --no-install-recommends apt-transport-https \
      ca-certificates \
      curl \
      gnupg-agent \
      software-properties-common
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | apt-key add -
    add-apt-repository \
      "deb [arch=amd64] https://download.docker.com/linux/ubuntu \
      $(lsb_release -cs) \
      stable"
    apt-get update
    apt-get install -y docker-ce
    usermod -aG docker vagrant

    # install go
    wget -q https://dl.google.com/go/go1.15.linux-amd64.tar.gz
    tar -xvf go1.15.linux-amd64.tar.gz
    mv go /usr/local
    rm -f go1.15.linux-amd64.tar.gz

    # install nodejs (for docs)
    curl -sL https://deb.nodesource.com/setup_11.x | bash -
    apt-get install -y nodejs

    # install etc
    #apt install \
    #    git make gcc libc-dev build-base curl jq file gmp-dev clang

    # cleanup
    apt-get autoremove -y

    # set env variables
    echo 'export GOROOT=/usr/local/go' >> /home/vagrant/.bash_profile
    echo 'export GOPATH=/home/vagrant/go' >> /home/vagrant/.bash_profile
    echo 'export PATH=$PATH:$GOROOT/bin:$GOPATH/bin' >> /home/vagrant/.bash_profile
    echo 'export LC_ALL=en_US.UTF-8' >> /home/vagrant/.bash_profile
    echo 'cd $GOPATH/src/github.com/line/ostracon' >> /home/vagrant/.bash_profile

    mkdir -p /home/vagrant/go/bin
    mkdir -p /home/vagrant/go/src/github.com/line
    ln -s /vagrant /home/vagrant/go/src/github.com/line/ostracon

    chown -R vagrant:vagrant /home/vagrant/go
    chown vagrant:vagrant /home/vagrant/.bash_profile

    # get all deps and tools, ready to install/test
    su - vagrant  -c 'source /home/vagrant/.bash_profile'
    # XXX Should remove "make tools": https://github.com/line/ostracon/commit/c6e0d20d4bf062921fcc1eb5b2399447a7d2226e#diff-76ed074a9305c04054cdebb9e9aad2d818052b07091de1f20cad0bbac34ffb52
    #su - vagrant -c 'cd /home/vagrant/go/src/github.com/line/ostracon && make tools'
    su - vagrant -c 'cd /home/vagrant/go/src/github.com/line/ostracon'
  SHELL
end
