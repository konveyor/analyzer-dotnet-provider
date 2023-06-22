FROM registry.access.redhat.com/ubi8/dotnet-70 AS builder
USER root
RUN microdnf -y install dnf
RUN dnf -y install 'dnf-command(config-manager)'
RUN dnf config-manager --set-enabled ubi-8-codeready-builder-rpms
RUN dnf -y install https://dl.fedoraproject.org/pub/epel/epel-release-latest-8.noarch.rpm
RUN dnf -y install mono-complete
USER default
RUN curl -s -L -O https://github.com/OmniSharp/omnisharp-roslyn/archive/refs/tags/v1.39.6.tar.gz \
  && tar -xf v1.39.6.tar.gz
RUN cd omnisharp-roslyn-1.39.6 && ./build.sh --target Build --use-global-dotnet-sdk

FROM registry.access.redhat.com/ubi8/dotnet-70
USER root
RUN microdnf -y install dotnet-sdk-2.1.x86_64 dotnet-sdk-2.1.5xx.x86_64 dotnet-sdk-3.0.x86_64 dotnet-sdk-3.1.x86_64 dotnet-sdk-5.0.x86_64 dotnet-sdk-6.0.x86_64 && microdnf clean all && rm -rf /var/cache/yum
USER default
COPY --from=builder /opt/app-root/src/omnisharp-roslyn-1.39.6/bin/Release/OmniSharp.Stdio.Driver/net6.0/ /opt/app-root/omnisharp
COPY --from=builder /opt/app-root/src/omnisharp-roslyn-1.39.6/bin/Release/OmniSharp.Http.Driver/net6.0/ /opt/app-root/omnisharp-http
ENTRYPOINT ["dotnet", "/opt/app-root/omnisharp/OmniSharp.dll"]
