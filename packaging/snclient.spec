Name:          snclient
Version:       UNSET
Release:       0
License:       MIT
Packager:      Sven Nierlein <sven.nierlein@consol.de>
Vendor:        Labs Consol
URL:           https://github.com/sni/snclient/
Source0:       snclient-%{version}.tar.gz
Group:         Applications/System
Summary:       Monitoring Agent
Requires:      logrotate

%description
SNClient is a Cross platform general purpose monitoring agent designed
as replacement for NRPE and NSClient++.
It supports Prometheus, NRPE and a REST API HTTP(s) protocol to run checks.

%prep
%setup -q

%build

%install
%{__mkdir_p} -m 0755 %{buildroot}/usr/bin
%{__install} -D -m 0644 -p snclient %{buildroot}/usr/bin/snclient

%{__mkdir_p} -m 0755 %{buildroot}/etc/snclient
%{__install} -D -m 0644 -p snclient.ini cacert.pem server.crt server.key %{buildroot}/etc/snclient

%{__mkdir_p} -m 0755 %{buildroot}/etc/logrotate.d
%{__install} -D -m 0644 -p snclient.logrotate %{buildroot}/etc/logrotate.d/snclient

%{__mkdir_p} -m 0755 %{buildroot}/lib/systemd/system
%{__install} -D -m 0644 -p snclient.service %{buildroot}/lib/systemd/system/snclient.service

%{__mkdir_p} -m 0755 %{buildroot}/usr/share/snclient
%{__install} -D -m 0644 -p README.md LICENSE %{buildroot}/usr/share/snclient

%{__mkdir_p} -m 0755 %{buildroot}/usr/share/man/man1
%{__install} -D -m 0644 -p snclient.1 %{buildroot}/usr/share/man/man1/snclient.1
gzip -n -9 %{buildroot}/usr/share/man/man1/snclient.1

%{__mkdir_p} -m 0755 %{buildroot}/usr/share/man/man8
%{__install} -D -m 0644 -p snclient.8 %{buildroot}/usr/share/man/man8/snclient.8
gzip -n -9 %{buildroot}/usr/share/man/man8/snclient.8


%post
case "$*" in
  1)
    # First installation
    systemctl daemon-reload &>/dev/null || true
    systemctl start snclient.service &>/dev/null || true
  ;;
  2)
    # Upgrading
    systemctl daemon-reload &>/dev/null || true
    systemctl condrestart snclient.service &>/dev/null || true
  ;;
  *) echo case "$*" not handled in post
esac

%preun
case "$*" in
  0)
    # Uninstall
    systemctl stop snclient.service &>/dev/null || true
  ;;
  1)
    # Upgrade, don't do anything
  ;;
  *) echo case "$*" not handled in preun
esac
exit 0

%postun
case "$*" in
  0)
    # post uninstall
    systemctl daemon-reload &>/dev/null || true
    ;;
  1)
    # post update
    ;;
  *) echo case "$*" not handled in postun
esac
exit 0

%clean
%{__rm} -rf %{buildroot}

%files
%defattr(-,root,root)
%attr(0755,root,root) /usr/bin/snclient
%attr(0644,root,root) /lib/systemd/system/snclient.service
%config(noreplace) /etc/snclient
%config(noreplace) /etc/logrotate.d/snclient
%doc /usr/share/snclient/README.md
%doc /usr/share/snclient/LICENSE
%doc /usr/share/man/man1/snclient.1.gz
%doc /usr/share/man/man8/snclient.8.gz

%changelog
