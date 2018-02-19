#!/usr/bin/env python
import subprocess
import os

import depflow

flow = depflow.Depflow()


@flow.depends(
    depflow.no_file('{}/bin/protoc-gen-go'.format(os.environ['GOPATH'])))
def protocgengo():
    subprocess.check_call([
        'go',
        'get',
        '-u',
        'github.com/golang/protobuf/protoc-gen-go',
    ])


for source in ('config', 'messages', 'storage', 'types'):
    sflow = flow.scope('proto', source)
    spath = 'trezor-common/protob/{}.proto'.format(source)

    @flow.depends(
        depflow.no_file('{}.pb.go'.format(source)),
        depflow.file_hash(spath),
    )
    def proto():
        env = os.environ.copy()
        env['PATH'] += ':{}/bin'.format(os.environ['GOPATH'])
        subprocess.check_call([
            'protoc',
            '--go_out=import_path=trezor:.',
            '-I/usr/include',
            '-I./trezor-common/protob',
            spath,
        ], env=env)