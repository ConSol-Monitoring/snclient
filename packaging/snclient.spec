Name:          snclient
Version:       UNSET
Release:       0
License:       MIT
Packager:      Sven Nierlein <sven.nierlein@consol.de>
Vendor:        Labs Consol
URL:           https://github.com/sni/snclient/
Source0:       snclient-%{version}.tar.gz
Group:         Applications/Monitoring
Summary:       Monitoring Agent
%if 0%{?systemd_requires}
%systemd_requires
%endif

%description
SNClient is a Cross platform general purpose monitoring agent mainly for Naemon
as replacement for NSClient++.
It supports Prometheus, NRPE and a REST API HTTP(s) protocol to run checks.

%prep
%setup -q

%build

%install
%{__mkdir_p} -m 0755 %{buildroot}/usr/bin
%{__mkdir_p} -m 0755 %{buildroot}/etc/snclient
%{__mkdir_p} -m 0755 %{buildroot}/lib/systemd/system
%{__install} -D -m 0644 -p snclient.service %{buildroot}/lib/systemd/system/snclient.service
%{__install} -D -m 0644 -p snclient %{buildroot}/usr/bin/snclient
%{__install} -D -m 0644 -p snclient.ini cacert.pem server.crt server.key %{buildroot}/etc/snclient

%clean
%{__rm} -rf %{buildroot}

%files
%defattr(-,root,root)
%attr(0755,root,root) /usr/bin/snclient
%attr(0644,root,root) /lib/systemd/system/snclient.service
%config(noreplace) /etc/snclient

%changelog