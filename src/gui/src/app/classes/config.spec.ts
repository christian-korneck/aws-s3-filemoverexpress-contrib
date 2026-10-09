import { describe, it, expect } from 'vitest';
import * as ConfigInterfaces from '@app/interfaces/config';
import {
    Checksums,
    General,
    HotFolders,
    HotFoldersTransferConfigurations,
    Logging,
    Paths,
    Protocols,
    Reports,
    S3Config,
    TransferProfile,
} from '@classes/config';
import * as proto from '@gen/es/fme/v1/config_pb';
import { REDACTED_SECRET } from '@app/constants/providers.constants';

const txpData: ConfigInterfaces.ConfigTransferProfile = {
    name: 'name',
    bucket: 'bucket',
    region: 'us-west-2',
    profile: 'profile-name',
    accelerated: false,
    fileOrder: [],
    enableMetadataFilter: false,
    storageClass: 'STANDARD',
    paths: Paths.fromJson({local: '', remote: ''}),
    chunkSize: 25,
    threads: 10,
    maxAge: '',
    filter: '',
    checksums: Checksums.fromJson({algorithm: 'none', enabled: false}),
    autoTuning: true,
    endpoint: '',
};

describe('General', () => {
    const data: ConfigInterfaces.ConfigGeneral = {
        noSleep: true,
        retryCount: 3,
        maxActiveTransfers: 20,
        autoMaxActiveTransfers: true,
        maxActiveChecksums: 8,
        targetBandwidth: 150,
    };

    function validate(gen: General) {
        expect(gen).toBeTruthy();
        expect(gen.noSleep).toEqual(data.noSleep);
        expect(gen.retryCount).toEqual(data.retryCount);
        expect(gen.maxActiveTransfers).toEqual(data.maxActiveTransfers);
        expect(gen.autoMaxActiveTransfers).toEqual(data.autoMaxActiveTransfers);
        expect(gen.maxActiveChecksums).toEqual(data.maxActiveChecksums);
        expect(gen.targetBandwidth).toEqual(data.targetBandwidth);
    }

    it('should create', () => {
        const gen = new General(
            data.noSleep,
            data.retryCount,
            data.maxActiveTransfers,
            data.autoMaxActiveTransfers,
            data.maxActiveChecksums,
            data.targetBandwidth,
        );

        validate(gen);
    });

    it('should convert from protobuf', () => {
        const pb = General.jsonToProtobuf(data);
        const gen = General.fromProtobuf(pb);
        validate(gen);
    });

    it('should convert from json', () => {
        const gen = General.fromJson(data);
        validate(gen);
    });
});

describe('HotFolders', () => {
    const data: ConfigInterfaces.ConfigHotFolders = {
        name: 'My HotFolder',
        enabled: true,
        localSourceFolder: '/path/to/hotfolder',
        remoteConfigurations: [
            {
                remoteConfigurationName: 'txp-name',
                s3DestinationFolder: 'prefix/in/bucket',
            },
        ],
    };

    function validate(hf: HotFolders) {
        expect(hf).toBeTruthy();
        expect(hf.name).toEqual(data.name);
        expect(hf.enabled).toEqual(data.enabled);
        expect(hf.localSourceFolder).toEqual(data.localSourceFolder);
    }

    it('should create', () => {
        const hf = new HotFolders(
            data.name,
            data.enabled,
            data.localSourceFolder,
            data.remoteConfigurations.map((itm) => HotFoldersTransferConfigurations.fromJson(itm)),
        );

        validate(hf);
    });

    it('should convert from protobuf', () => {
        const pb = HotFolders.jsonToProtobuf(data);
        const hf = HotFolders.fromProtobuf(pb);
        validate(hf);
    });

    it('should convert from json', () => {
        const hf = HotFolders.fromJson(data);
        validate(hf);
    });
});

describe('HotFolderTransferConfigurations', () => {
    const data: ConfigInterfaces.HotFolderTransferConfigurations = {
        remoteConfigurationName: 'txp-name',
        s3DestinationFolder: 'prefix/in/bucket',
    };

    it('should create', () => {
        const hf = new HotFoldersTransferConfigurations(
            data.remoteConfigurationName,
            data.s3DestinationFolder,
        );

        expect(hf).toBeTruthy();
        expect(hf.remoteConfigurationName).toEqual(data.remoteConfigurationName);
        expect(hf.s3DestinationFolder).toEqual(data.s3DestinationFolder);
    });

    it('should convert from protobuf', () => {
        const pb = HotFoldersTransferConfigurations.jsonToProtobuf(data);
        const hf = HotFoldersTransferConfigurations.fromProtobuf(pb);

        expect(hf).toBeTruthy();
        expect(hf.remoteConfigurationName).toEqual(data.remoteConfigurationName);
        expect(hf.s3DestinationFolder).toEqual(data.s3DestinationFolder);
    });

    it('should convert from json', () => {
        const hf = HotFoldersTransferConfigurations.fromJson(data);

        expect(hf).toBeTruthy();
        expect(hf.remoteConfigurationName).toEqual(data.remoteConfigurationName);
        expect(hf.s3DestinationFolder).toEqual(data.s3DestinationFolder);
    });
});

describe('Logging', () => {
    const data: ConfigInterfaces.ConfigLogging = {
        directory: '/path/to/log/dir',
        severity: 'INFO',
        maxSize: 10,
        maxAge: 7,
        compress: true,
    };

    function validate(log: Logging) {
        expect(log).toBeTruthy();
        expect(log.directory).toEqual(data.directory);
        expect(log.severity).toEqual(data.severity);
        expect(log.maxSize).toEqual(data.maxSize);
        expect(log.maxAge).toEqual(data.maxAge);
        expect(log.compress).toEqual(data.compress);
    }

    it('should create', () => {
        const log = new Logging(
            data.compress,
            data.directory,
            data.maxAge,
            data.maxSize,
            data.severity,
        );

        validate(log);
    });

    it('should convert from protobuf', () => {
        const pb = Logging.jsonToProtobuf(data);
        const log = Logging.fromProtobuf(pb);
        validate(log);
    });

    it('should convert from json', () => {
        const log = Logging.fromJson(data);
        validate(log);
    });
});

describe('Reports', () => {
    const reportDir = '/path/to/reports';
    it('should create', () => {
        const report = new Reports(reportDir);

        expect(report).toBeTruthy();
        expect(report.directory).toEqual(reportDir);
    });

    it('should convert from protobuf', () => {
        const pb = Reports.jsonToProtobuf({directory: reportDir});
        const report = Reports.fromProtobuf(pb);

        expect(report).toBeTruthy();
        expect(report.directory).toEqual(reportDir);
    });

    it('should convert from json', () => {
        const report = Reports.fromJson({directory: reportDir});

        expect(report).toBeTruthy();
        expect(report.directory).toEqual(reportDir);
    });
});

describe('Protocols', () => {
    const s3 = new S3Config({name: txpData});

    it('should create', () => {
        const p = new Protocols(s3);
        expect(p).toBeTruthy();
    });

    it('should convert from protobuf', () => {
        const s3Data = {
            transferProfiles: {name: txpData},
        };
        const pb = Protocols.jsonToProtobuf({s3: s3Data});

        const p = Protocols.fromProtobuf(pb);
        expect(p).toBeTruthy();
    });
});


describe('S3Config', () => {
    function validateS3(s3: S3Config) {
        for (const [name, txp] of Object.entries(s3['transferProfiles'])) {
            expect(txp).toBeTruthy();
            expect(txp.name).toEqual('name');
            expect(txp.name).toEqual(name);
            expect(txp.bucket).toEqual('bucket');
            expect(txp.region).toEqual('us-west-2');
            expect(txp.profile).toEqual('profile-name');
            expect(txp.accelerated).toEqual(false);
            expect(txp.fileOrder).toEqual([]);
            expect(txp.enableMetadataFilter).toEqual(false);
            expect(txp.storageClass).toEqual('STANDARD');
            expect(txp.paths).toEqual(txpData.paths);
            expect(txp.chunkSize).toEqual(25);
            expect(txp.threads).toEqual(10);
            expect(txp.maxAge).toEqual('');
            expect(txp.filter).toEqual('');
            expect(txp.checksums).toEqual(txpData.checksums);
            expect(txp.autoTuning).toEqual(true);
            expect(txp.endpoint).toEqual('');
        }
    }

    it('should create', () => {
        const s3 = new S3Config({name: txpData});
        validateS3(s3);
    });

    it('should convert from protobuf', () => {
        const s3Data = {
            transferProfiles: {name: txpData},
        };
        const pb = S3Config.jsonToProtobuf(s3Data);

        const s3 = S3Config.fromProtobuf(pb);
        validateS3(s3);
    });

    it('should convert from json', () => {
        const s3 = S3Config.fromJson({transferProfiles: {name: txpData}});
        validateS3(s3);
    });
});

describe('TransferProfile', () => {
    function validateTxp(t: TransferProfile) {
        expect(t).toBeTruthy();
        expect(t.name).toEqual(txpData.name);
        expect(t.bucket).toEqual(txpData.bucket);
        expect(t.region).toEqual(txpData.region);
        expect(t.profile).toEqual(txpData.profile);
        expect(t.accelerated).toEqual(txpData.accelerated);
        expect(t.fileOrder).toEqual(txpData.fileOrder);
        expect(t.enableMetadataFilter).toEqual(txpData.enableMetadataFilter);
        expect(t.storageClass).toEqual(txpData.storageClass);
        expect(t.paths).toEqual(txpData.paths);
        expect(t.chunkSize).toEqual(txpData.chunkSize);
        expect(t.threads).toEqual(txpData.threads);
        expect(t.maxAge).toEqual(txpData.maxAge);
        expect(t.filter).toEqual(txpData.filter);
        expect(t.checksums).toEqual(txpData.checksums);
        expect(t.autoTuning).toEqual(txpData.autoTuning);
        expect(t.endpoint).toEqual(txpData.endpoint);
    }

    it('should create', () => {
        const t = new TransferProfile(
            txpData.name,
            txpData.bucket,
            txpData.region,
            txpData.profile,
            txpData.accelerated,
            txpData.fileOrder,
            txpData.enableMetadataFilter,
            txpData.storageClass,
            txpData.paths,
            txpData.chunkSize,
            txpData.threads,
            txpData.maxAge,
            txpData.filter,
            txpData.checksums,
            txpData.autoTuning,
            txpData.endpoint,
        );

        validateTxp(t);
    });

    it('should convert from protobuf', () => {
        const pb = TransferProfile.jsonToProtobuf(txpData);

        const t = TransferProfile.fromProtobuf(pb);
        validateTxp(t);
    });

    it('should convert from protobuf without paths', () => {
        const pb = TransferProfile.jsonToProtobuf(txpData);
        pb.paths = undefined;

        setCommonPbTxpData(pb, txpData);

        const t = TransferProfile.fromProtobuf(pb);
        validateTxp(t);
    });

    it('should convert from json', () => {
        const t = TransferProfile.fromJson(txpData);
        validateTxp(t);
    });
});

describe('Paths', () => {
    it('should create', () => {
        const p = new Paths('/tmp', 'prefixA/');
        expect(p).toBeTruthy();
        expect(p.local).toEqual('/tmp');
        expect(p.remote).toEqual('prefixA/');
    });

    it('should convert from protobuf', () => {
        const pb = Paths.jsonToProtobuf({
            local: '/tmp',
            remote: 'prefixA/',
        });

        const p = Paths.fromProtobuf(pb);
        expect(p).toBeTruthy();
        expect(p.local).toEqual('/tmp');
        expect(p.remote).toEqual('prefixA/');
    });

    it('should convert from json', () => {
        const p = Paths.fromJson({
            local: '/tmp',
            remote: 'prefixA/',
        });

        expect(p).toBeTruthy();
        expect(p.local).toEqual('/tmp');
        expect(p.remote).toEqual('prefixA/');
    });
});

describe('Checksums', () => {
    it('should create', () => {
        const c = new Checksums(false, 'none');
        expect(c).toBeTruthy();
        expect(c.enabled).toBeFalsy();
        expect(c.algorithm).toEqual('none');
    });

    it('should convert from protobuf', () => {
        const pb = Checksums.jsonToProtobuf({
            enabled: true,
            algorithm: 'md5',
        });
        const c = Checksums.fromProtobuf(pb);

        expect(c).toBeTruthy();
        expect(c.enabled).toBeTruthy();
        expect(c.algorithm).toEqual('md5');
    });

    it('should convert from json', () => {
        const c = Checksums.fromJson({enabled: true, algorithm: 'xxhash'});
        expect(c).toBeTruthy();
        expect(c.enabled).toBeTruthy();
        expect(c.algorithm).toEqual('xxhash');
    });
});

function setCommonPbTxpData(pb: proto.TransferProfile, inputConfig: ConfigInterfaces.ConfigTransferProfile) {
    pb.name = inputConfig.name;
    pb.bucket = inputConfig.bucket;
    pb.region = inputConfig.region;
    pb.profile = inputConfig.profile;
    pb.accelerated = inputConfig.accelerated;
    pb.fileOrder = inputConfig.fileOrder;
    pb.enableMetadataFilter = inputConfig.enableMetadataFilter;
    pb.storageClass = inputConfig.storageClass;
    pb.chunkSize = inputConfig.chunkSize;
    pb.threads = inputConfig.threads;
    pb.maxAge = inputConfig.maxAge;
    pb.filter = inputConfig.filter;
    pb.autoTuning = inputConfig.autoTuning;
    pb.endpoint = inputConfig.endpoint;
}

describe('TransferProfile S3-compatible provider settings', () => {
    const minioData: ConfigInterfaces.ConfigTransferProfile = {
        ...txpData,
        name: 'minio-local',
        endpoint: 'http://nas.local:9000',
        authMethod: 'static-keys',
        provider: 'minio',
        pathStyle: true,
        accessKeyId: 'minioadmin',
        secretAccessKey: 's3cr3t',
        sessionToken: 'tok',
        tlsCaBundle: '/etc/ca.pem',
        tlsSkipVerify: true,
    };

    it('maps provider, keys and TLS settings to protobuf', () => {
        const pb = TransferProfile.jsonToProtobuf(minioData);
        expect(pb.provider).toBe(proto.S3Provider.MINIO);
        expect(pb.pathStyle).toBe(true);
        expect(pb.authMethod).toBe(proto.AuthMethod.STATIC_KEYS);
        expect(pb.staticCredentials?.accessKeyId).toBe('minioadmin');
        expect(pb.staticCredentials?.secretAccessKey).toBe('s3cr3t');
        expect(pb.staticCredentials?.sessionToken).toBe('tok');
        expect(pb.tls?.caBundle).toBe('/etc/ca.pem');
        expect(pb.tls?.skipVerify).toBe(true);
    });

    it('round-trips through protobuf', () => {
        const txp = TransferProfile.fromProtobuf(TransferProfile.jsonToProtobuf(minioData));
        expect(txp.provider).toBe('minio');
        expect(txp.pathStyle).toBe(true);
        expect(txp.authMethod).toBe('static-keys');
        expect(txp.accessKeyId).toBe('minioadmin');
        expect(txp.secretAccessKey).toBe('s3cr3t');
        expect(txp.sessionToken).toBe('tok');
        expect(txp.tlsCaBundle).toBe('/etc/ca.pem');
        expect(txp.tlsSkipVerify).toBe(true);
    });

    it('omits static credentials unless the auth method is static-keys', () => {
        const pb = TransferProfile.jsonToProtobuf({...minioData, authMethod: 'aws-profile'});
        expect(pb.authMethod).toBe(proto.AuthMethod.AWS_PROFILE);
        expect(pb.staticCredentials).toBeUndefined();
    });

    it('treats legacy profiles as Amazon S3 with safe defaults', () => {
        const legacy = TransferProfile.fromJson(txpData);
        expect(legacy.provider).toBe('aws');
        expect(legacy.pathStyle).toBe(false);
        expect(legacy.accessKeyId).toBe('');
        expect(legacy.tlsSkipVerify).toBe(false);

        const fromWire = TransferProfile.fromProtobuf(TransferProfile.jsonToProtobuf(txpData));
        expect(fromWire.provider).toBe('aws');
        expect(fromWire.authMethod).toBe('aws-profile');

        // Configurations written before the provider field existed arrive as UNSPECIFIED.
        const legacyMessage = TransferProfile.jsonToProtobuf(txpData);
        legacyMessage.provider = proto.S3Provider.UNSPECIFIED;
        legacyMessage.tls = undefined;
        const unspecified = TransferProfile.fromProtobuf(legacyMessage);
        expect(unspecified.provider).toBe('aws');
        expect(unspecified.tlsCaBundle).toBe('');
    });

    it('maps an unknown provider id to the generic S3-compatible provider', () => {
        const txp = TransferProfile.fromJson({...minioData, provider: 'bogus' as never});
        expect(txp.provider).toBe('other');
        expect(TransferProfile.jsonToProtobuf(txp).provider).toBe(proto.S3Provider.OTHER);
    });

    it('sends TLS settings but no static credentials for a named profile', () => {
        const pb = TransferProfile.jsonToProtobuf({...minioData, authMethod: 'aws-profile', profile: 'lab'});
        expect(pb.profile).toBe('lab');
        expect(pb.tls?.caBundle).toBe('/etc/ca.pem');
        expect(pb.tls?.skipVerify).toBe(true);
        expect(pb.staticCredentials).toBeUndefined();
    });

    it('sends an empty session token for long-lived access keys', () => {
        const pb = TransferProfile.jsonToProtobuf({...minioData, sessionToken: ''});
        expect(pb.staticCredentials?.secretAccessKey).toBe('s3cr3t');
        expect(pb.staticCredentials?.sessionToken).toBe('');
    });

    it('passes redacted secrets through unchanged so the daemon keeps the stored values', () => {
        const pb = TransferProfile.jsonToProtobuf({
            ...minioData,
            secretAccessKey: REDACTED_SECRET,
            sessionToken: REDACTED_SECRET,
        });
        expect(pb.staticCredentials?.secretAccessKey).toBe(REDACTED_SECRET);
        expect(pb.staticCredentials?.sessionToken).toBe(REDACTED_SECRET);

        const txp = TransferProfile.fromProtobuf(pb);
        expect(txp.secretAccessKey).toBe(REDACTED_SECRET);
        expect(txp.sessionToken).toBe(REDACTED_SECRET);
    });

    it('trims the endpoint', () => {
        const pb = TransferProfile.jsonToProtobuf({...minioData, endpoint: '  http://nas.local:9000 \n'});
        expect(pb.endpoint).toBe('http://nas.local:9000');
    });
});
