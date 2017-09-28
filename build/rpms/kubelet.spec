Name: kubelet
Version: OVERRIDE_THIS
Release: 00
License: ASL 2.0
Summary: Container Cluster Manager

URL: https://kubernetes.io

%{?systemd_requires}
BuildRequires: systemd
Requires: iptables >= 1.4.21
Requires: kubernetes-cni >= 0.5.1
Requires: socat
Requires: util-linux
Requires: ethtool
Requires: iproute
Requires: ebtables

%description
The node agent of Kubernetes, the container cluster manager.

%install

install -m 755 -d %{buildroot}%{_sysconfdir}/kubernetes/manifests/
install -m 755 -d %{buildroot}%{_bindir}
install -p -m 755 -t %{buildroot}%{_bindir} kubelet
# install service file
install -d -m 0755 %{buildroot}%{_unitdir}
install -m 0644 -t %{buildroot}%{_unitdir} kubelet.service

%post
%systemd_post kubelet

%preun
%systemd_preun kubelet

%postun
%systemd_postun kubelet

%files
%{_bindir}/kubelet
%{_unitdir}/kubelet.service
%{_sysconfdir}/kubernetes/manifests/
