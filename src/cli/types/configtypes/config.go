package configtypes

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/awslabs/filemoverexpress/constants"
	"github.com/awslabs/filemoverexpress/logger"
	"github.com/awslabs/filemoverexpress/types/pbtypes/fme/v1"
	"github.com/awslabs/filemoverexpress/utils/safeconv"
)

const (
	// AuthMethodUnspecified is the proto3 default — treated identically to AuthMethodAWSProfile.
	AuthMethodUnspecified AuthMethod = 0
	// AuthMethodAWSProfile uses existing AWS named profile credential resolution.
	AuthMethodAWSProfile AuthMethod = 1
	// AuthMethodOIDC uses OIDC → AssumeRoleWithWebIdentity for credential acquisition.
	AuthMethodOIDC AuthMethod = 2
	// AuthMethodStaticKeys uses an access key ID / secret access key stored in the configuration file.
	AuthMethodStaticKeys AuthMethod = 3
)

// When updating the tag values, make sure to make the corresponding updates for the constants in config/config/config-keys.go
// revive:disable:max-public-structs
type (
	// AuthMethod represents the credential acquisition strategy for a TransferProfile.
	AuthMethod int32

	FmeConfig struct {
		General          General                   `koanf:"general" yaml:"general"`
		Logging          Logging                   `koanf:"logging" yaml:"logging"`
		Reports          Reports                   `koanf:"reports" yaml:"reports"`
		APIServer        APIServer                 `koanf:"apiServer" yaml:"apiServer"`
		Protocols        ProtocolList              `koanf:"protocols" yaml:"protocols"`
		UploadHotFolders []UploadHotFolderSettings `koanf:"hotFolders" yaml:"hotFolders"`
	}
	General struct {
		NoSleep                bool   `koanf:"noSleep" yaml:"noSleep"`
		RetryCount             uint32 `koanf:"retryCount" yaml:"retryCount"`
		MaxActiveChecksums     int32  `koanf:"maxActiveChecksums" yaml:"maxActiveChecksums"`
		MaxActiveTransfers     int32  `koanf:"maxActiveTransfers" yaml:"maxActiveTransfers"`
		AutoMaxActiveTransfers bool   `koanf:"autoMaxActiveTransfers" yaml:"autoMaxActiveTransfers"`
		TargetBandwidth        int32  `koanf:"targetBandwidth" yaml:"targetBandwidth"`
	}
	Logging struct {
		Directory string `koanf:"directory" yaml:"directory"`
		Severity  string `koanf:"severity" yaml:"severity"`
		MaxSize   int    `koanf:"maxSize" yaml:"maxSize"`
		MaxAge    int    `koanf:"maxAge" yaml:"maxAge"`
		Compress  bool   `koanf:"compress" yaml:"compress"`
	}
	Reports struct {
		Directory string `koanf:"directory" yaml:"directory"`
	}
	APIServer struct {
		Enabled         bool                        `koanf:"enabled" yaml:"enabled"`
		TLSSettings     APIServerTLSSettings        `koanf:"tls" yaml:"tls"`
		RemoteSettings  APIServerRemoteSettings     `koanf:"remote" yaml:"remote"`
		BlockedPathList []string                    `koanf:"blockedPaths" yaml:"blockedPaths"`
		AllowedOrigins  []string                    `koanf:"allowedOrigins" yaml:"allowedOrigins"`
		Permissions     APIServerPermissionSettings `koanf:"permissions" yaml:"permissions"`
	}
	APIServerRemoteSettings struct {
		Enabled      bool     `koanf:"enabled" yaml:"enabled"`
		PreSharedKey string   `koanf:"key" yaml:"key"`
		Ports        []uint32 `koanf:"ports" yaml:"ports"`
		Address      string   `koanf:"address" yaml:"address"`
	}
	APIServerTLSSettings struct {
		Enabled         bool   `koanf:"enabled" yaml:"enabled"`
		CertificateFile string `koanf:"certificateFile" yaml:"certificateFile"`
		KeyFile         string `koanf:"keyFile" yaml:"keyFile"`
	}
	APIServerPermissionSettings struct {
		AllowUIConfiguration    bool `koanf:"allowUIConfiguration" yaml:"allowUIConfiguration"`
		AllowLocalRenameDelete  bool `koanf:"allowLocalRenameDelete" yaml:"allowLocalRenameDelete"`
		AllowRemoteRenameDelete bool `koanf:"allowRemoteRenameDelete" yaml:"allowRemoteRenameDelete"`
	}
	ProtocolList struct {
		S3 S3ProtocolConfig `koanf:"s3" yaml:"s3"`
	}
	TransferProfile struct {
		Name     string `koanf:"name" yaml:"name"`
		Bucket   string `koanf:"bucket" yaml:"bucket"`
		Region   string `koanf:"region" yaml:"region"`
		Profile  string `koanf:"profile" yaml:"profile"`
		Endpoint string `koanf:"endpoint" yaml:"endpoint"`

		Filter     string           `koanf:"filter" yaml:"filter"`
		AutoTuning bool             `koanf:"autoTuning" yaml:"autoTuning"`
		Checksums  ChecksumSettings `koanf:"checksums" yaml:"checksums"`
		ChunkSize  int32            `koanf:"chunkSize" yaml:"chunkSize"`
		Threads    int              `koanf:"threads" yaml:"threads"`
		MaxAge     string           `koanf:"maxAge" yaml:"maxAge"`

		Accelerated bool     `koanf:"accelerated" yaml:"accelerated"`
		FileOrder   []string `koanf:"fileOrder" yaml:"fileOrder"`

		EnableMetadataFilter bool   `koanf:"enableMetadataFilter" yaml:"enableMetadataFilter"`
		StorageClass         string `koanf:"storageClass" yaml:"storageClass"`

		Paths PathsSettings `koanf:"paths" yaml:"paths"`

		AuthMethod AuthMethod  `koanf:"authMethod" yaml:"authMethod"`
		OIDCConfig *OIDCConfig `koanf:"oidcConfig" yaml:"oidcConfig"`

		// Provider selects the S3 implementation; empty means Amazon S3 (see providers.go).
		Provider          S3Provider          `koanf:"provider" yaml:"provider,omitempty"`
		PathStyle         bool                `koanf:"pathStyle" yaml:"pathStyle,omitempty"`
		StaticCredentials *StaticCredentials  `koanf:"staticCredentials" yaml:"staticCredentials,omitempty"`
		TLS               EndpointTLSSettings `koanf:"tls" yaml:"tls,omitempty"`
	}
	StaticCredentials struct {
		AccessKeyID     string `koanf:"accessKeyId" yaml:"accessKeyId"`
		SecretAccessKey string `koanf:"secretAccessKey" yaml:"secretAccessKey"`
		SessionToken    string `koanf:"sessionToken" yaml:"sessionToken,omitempty"`
	}
	EndpointTLSSettings struct {
		CABundle   string `koanf:"caBundle" yaml:"caBundle"`
		SkipVerify bool   `koanf:"skipVerify" yaml:"skipVerify"`
	}
	ChecksumSettings struct {
		Enabled   bool                        `koanf:"enabled" yaml:"enabled"`
		Algorithm constants.ChecksumAlgorithm `koanf:"algorithm" yaml:"algorithm"`
	}
	UploadHotFolderSettings struct {
		Enabled              bool                            `koanf:"enabled" yaml:"enabled"`
		LocalSourceFolder    string                          `koanf:"localSourceFolder" yaml:"localSourceFolder"`
		Name                 string                          `koanf:"name" yaml:"name"`
		ForceInitialUpload   bool                            `koanf:"forceInitialUpload" yaml:"forceInitialUpload"`
		RemoteConfigurations []HotFolderRemoteConfigurations `koanf:"remoteConfigurations" yaml:"remoteConfigurations"`
	}
	HotFolderRemoteConfigurations struct {
		RemoteConfigurationName string `koanf:"remoteConfigurationName" yaml:"remoteConfigurationName"`
		S3DestinationFolder     string `koanf:"s3DestinationFolder" yaml:"s3DestinationFolder"`
	}
	PathsSettings struct {
		Local  string `koanf:"local" yaml:"local"`
		Remote string `koanf:"remote" yaml:"remote"`
	}
	S3ProtocolConfig struct {
		TransferProfiles map[string]TransferProfile `koanf:"transferProfiles" yaml:"transferProfiles"`
	}
	OIDCConfig struct {
		IssuerURL              string   `koanf:"issuerUrl" yaml:"issuerUrl"`
		ClientID               string   `koanf:"clientId" yaml:"clientId"`
		RoleARN                string   `koanf:"roleArn" yaml:"roleArn"`
		Scopes                 []string `koanf:"scopes" yaml:"scopes"`
		PersistSession         bool     `koanf:"persistSession" yaml:"persistSession"`
		CustomCABundle         string   `koanf:"customCaBundle" yaml:"customCaBundle"`
		SessionDurationSeconds int32    `koanf:"sessionDurationSeconds" yaml:"sessionDurationSeconds"`
	}
)

// String returns a string format of the provided TransferProfile struct, including all of its properties
func (v *TransferProfile) String() string {
	var out []string
	e := reflect.ValueOf(v).Elem()

	for i := 0; i < e.NumField(); i++ {
		out = append(out, fmt.Sprintf("    %s: %v", e.Type().Field(i).Name, e.Field(i).Interface()))
	}

	sort.Strings(out)
	return strings.Join(out, "\n")
}

func (fmeConfig FmeConfig) GetTransferProfile(profileName string) (TransferProfile, error) {
	for name, transferProfile := range fmeConfig.Protocols.S3.TransferProfiles {
		if name == profileName {
			return transferProfile, nil
		}
	}

	return TransferProfile{}, errors.New(strNoSuchTransferProfile + profileName)
}

// Redacted returns a copy of the configuration with secret values replaced by a placeholder. Use it
// for anything that leaves the daemon host, such as support files.
func (fmeConfig FmeConfig) Redacted() FmeConfig {
	out := fmeConfig
	if out.APIServer.RemoteSettings.PreSharedKey != "" {
		out.APIServer.RemoteSettings.PreSharedKey = RedactedSecret
	}
	out.Protocols.S3.TransferProfiles = make(map[string]TransferProfile, len(fmeConfig.Protocols.S3.TransferProfiles))
	for name, transferProfile := range fmeConfig.Protocols.S3.TransferProfiles {
		transferProfile.StaticCredentials = transferProfile.StaticCredentials.Redacted()
		out.Protocols.S3.TransferProfiles[name] = transferProfile
	}
	return out
}

func (fmeConfig FmeConfig) ToGRPCProtobuf() *fmev1.GRPCFmeConfig {
	transferProfiles := TransferProfilesToProtobuf(fmeConfig.Protocols.S3.TransferProfiles)
	uploadHotFolders := HotFoldersToProtobuf(fmeConfig.UploadHotFolders)

	maxSize, err := safeconv.IntToInt32(fmeConfig.Logging.MaxSize)
	if err != nil {
		logger.Error("Invalid MaxSize value %d: %v, using default", fmeConfig.Logging.MaxSize, err)
		maxSize = 100
	}

	maxAge, err := safeconv.IntToInt32(fmeConfig.Logging.MaxAge)
	if err != nil {
		logger.Error("Invalid MaxAge value %d: %v, using default", fmeConfig.Logging.MaxAge, err)
		maxAge = 28
	}

	return &fmev1.GRPCFmeConfig{
		General: &fmev1.GeneralSettings{
			NoSleep:                fmeConfig.General.NoSleep,
			RetryCount:             fmeConfig.General.RetryCount,
			MaxActiveTransfers:     fmeConfig.General.MaxActiveTransfers,
			AutoMaxActiveTransfers: fmeConfig.General.AutoMaxActiveTransfers,
			MaxActiveChecksums:     fmeConfig.General.MaxActiveChecksums,
			TargetBandwidth:        fmeConfig.General.TargetBandwidth,
		},
		Logging: &fmev1.LoggingSettings{
			Directory: fmeConfig.Logging.Directory,
			Severity:  fmeConfig.Logging.Severity,
			MaxSize:   maxSize,
			MaxAge:    maxAge,
			Compress:  fmeConfig.Logging.Compress,
		},
		Reports: &fmev1.ReportsSettings{
			Directory: fmeConfig.Reports.Directory,
		},
		Protocols: &fmev1.Protocols{
			S3: &fmev1.S3Settings{
				TransferProfiles: transferProfiles,
			},
		},
		UploadHotFolders: uploadHotFolders,
	}
}

func FromGRPCProtobuf(newConfig *fmev1.GRPCFmeConfig, apiServerConfig APIServer) (FmeConfig, error) {
	var missing []string
	if newConfig.General == nil {
		missing = append(missing, "general")
	}
	if newConfig.Logging == nil {
		missing = append(missing, "logging")
	}
	if newConfig.Reports == nil {
		missing = append(missing, "reports")
	}
	if newConfig.Protocols == nil {
		missing = append(missing, "protocols")
	}
	if len(missing) > 0 {
		return FmeConfig{}, fmt.Errorf("missing required fields: %s", strings.Join(missing, ", "))
	}

	return FmeConfig{
		General: General{
			NoSleep:                newConfig.General.NoSleep,
			RetryCount:             newConfig.General.RetryCount,
			MaxActiveChecksums:     newConfig.General.MaxActiveChecksums,
			MaxActiveTransfers:     newConfig.General.MaxActiveTransfers,
			AutoMaxActiveTransfers: newConfig.General.AutoMaxActiveTransfers,
			TargetBandwidth:        newConfig.General.TargetBandwidth,
		},
		Logging: Logging{
			Directory: newConfig.Logging.Directory,
			Severity:  newConfig.Logging.Severity,
			MaxSize:   int(newConfig.Logging.MaxSize),
			MaxAge:    int(newConfig.Logging.MaxAge),
			Compress:  newConfig.Logging.Compress,
		},
		Reports: Reports{
			Directory: newConfig.Reports.Directory,
		},
		APIServer: apiServerConfig,
		Protocols: ProtocolList{
			S3: S3ProtocolConfig{
				TransferProfiles: TransferProfilesFromProtobuf(newConfig.Protocols.S3.TransferProfiles),
			},
		},
		UploadHotFolders: HotFoldersFromProtobuf(newConfig.UploadHotFolders),
	}, nil
}

func TransferProfilesFromProtobuf(profileList map[string]*fmev1.TransferProfile) (transferProfiles map[string]TransferProfile) {
	transferProfiles = make(map[string]TransferProfile)

	for name, transferProfile := range profileList {
		transferProfiles[name] = TransferProfileFromProtobuf(transferProfile)
	}

	return transferProfiles
}

// TransferProfileFromProtobuf converts a single wire profile. Nested messages can be nil when the
// message comes straight from a client, so every sub-message is guarded.
func TransferProfileFromProtobuf(transferProfile *fmev1.TransferProfile) TransferProfile {
	if transferProfile == nil {
		return TransferProfile{}
	}

	return TransferProfile{
		Name:                 transferProfile.Name,
		Bucket:               transferProfile.Bucket,
		Region:               transferProfile.Region,
		Profile:              transferProfile.Profile,
		Endpoint:             transferProfile.Endpoint,
		Accelerated:          transferProfile.Accelerated,
		FileOrder:            transferProfile.FileOrder,
		EnableMetadataFilter: transferProfile.EnableMetadataFilter,
		StorageClass:         transferProfile.StorageClass,
		Paths:                pathsFromProtobuf(transferProfile.Paths),
		AutoTuning:           transferProfile.AutoTuning,
		Checksums:            checksumsFromProtobuf(transferProfile.Checksums),
		ChunkSize:            transferProfile.ChunkSize,
		Filter:               transferProfile.Filter,
		Threads:              int(transferProfile.Threads),
		MaxAge:               transferProfile.MaxAge,
		AuthMethod:           AuthMethod(transferProfile.AuthMethod),
		OIDCConfig:           oidcConfigFromProtobuf(transferProfile.OidcConfig),
		Provider:             S3ProviderFromProtobuf(transferProfile.Provider),
		PathStyle:            transferProfile.PathStyle,
		StaticCredentials:    staticCredentialsFromProtobuf(transferProfile.AuthMethod, transferProfile.StaticCredentials),
		TLS:                  tlsFromProtobuf(transferProfile.Tls),
	}
}

func pathsFromProtobuf(paths *fmev1.PathsSettings) PathsSettings {
	if paths == nil {
		return PathsSettings{}
	}
	return PathsSettings{Local: paths.Local, Remote: paths.Remote}
}

func checksumsFromProtobuf(checksums *fmev1.ChecksumSettings) ChecksumSettings {
	if checksums == nil {
		return ChecksumSettings{}
	}
	return ChecksumSettings{Enabled: checksums.Enabled, Algorithm: constants.ChecksumAlgorithm(checksums.Algorithm)}
}

func oidcConfigFromProtobuf(oidc *fmev1.OIDCConfig) *OIDCConfig {
	if oidc == nil {
		return nil
	}
	return &OIDCConfig{
		IssuerURL:              oidc.IssuerUrl,
		ClientID:               oidc.ClientId,
		RoleARN:                oidc.RoleArn,
		Scopes:                 oidc.Scopes,
		PersistSession:         oidc.PersistSession,
		CustomCABundle:         oidc.CustomCaBundle,
		SessionDurationSeconds: oidc.SessionDurationSeconds,
	}
}

// staticCredentialsFromProtobuf keeps credentials only for the static-keys auth method, so that switching a
// profile to another method removes the stored secret instead of leaving it behind in the file.
func staticCredentialsFromProtobuf(authMethod fmev1.AuthMethod, creds *fmev1.StaticCredentials) *StaticCredentials {
	if creds == nil || authMethod != fmev1.AuthMethod_AUTH_METHOD_STATIC_KEYS {
		return nil
	}
	return &StaticCredentials{
		AccessKeyID:     creds.AccessKeyId,
		SecretAccessKey: creds.SecretAccessKey,
		SessionToken:    creds.SessionToken,
	}
}

func tlsFromProtobuf(tlsSettings *fmev1.EndpointTlsSettings) EndpointTLSSettings {
	if tlsSettings == nil {
		return EndpointTLSSettings{}
	}
	return EndpointTLSSettings{CABundle: tlsSettings.CaBundle, SkipVerify: tlsSettings.SkipVerify}
}

func TransferProfilesToProtobuf(transferProfiles map[string]TransferProfile) map[string]*fmev1.TransferProfile {
	out := make(map[string]*fmev1.TransferProfile)

	for name, transferProfile := range transferProfiles {
		out[name] = TransferProfileToProtobuf(transferProfile)
	}

	return out
}

// TransferProfileToProtobuf converts a single profile to its wire representation.
func TransferProfileToProtobuf(transferProfile TransferProfile) *fmev1.TransferProfile {
	threads, err := safeconv.IntToInt32(transferProfile.Threads)
	if err != nil {
		logger.Error("Invalid Threads value %d for profile '%s': %v, using default", transferProfile.Threads, transferProfile.Name, err)
		threads = 1 // Default thread count
	}

	return &fmev1.TransferProfile{
		Name:                 transferProfile.Name,
		Bucket:               transferProfile.Bucket,
		Region:               transferProfile.Region,
		Profile:              transferProfile.Profile,
		Endpoint:             transferProfile.Endpoint,
		Accelerated:          transferProfile.Accelerated,
		FileOrder:            transferProfile.FileOrder,
		EnableMetadataFilter: transferProfile.EnableMetadataFilter,
		StorageClass:         transferProfile.StorageClass,
		Paths: &fmev1.PathsSettings{
			Local:  transferProfile.Paths.Local,
			Remote: transferProfile.Paths.Remote,
		},
		AutoTuning: transferProfile.AutoTuning,
		Checksums: &fmev1.ChecksumSettings{
			Enabled:   transferProfile.Checksums.Enabled,
			Algorithm: string(transferProfile.Checksums.Algorithm),
		},
		ChunkSize:         transferProfile.ChunkSize,
		Filter:            transferProfile.Filter,
		Threads:           threads,
		MaxAge:            transferProfile.MaxAge,
		AuthMethod:        fmev1.AuthMethod(transferProfile.AuthMethod),
		OidcConfig:        oidcConfigToProtobuf(transferProfile.OIDCConfig),
		Provider:          transferProfile.Provider.ToProtobuf(),
		PathStyle:         transferProfile.PathStyle,
		StaticCredentials: staticCredentialsToProtobuf(transferProfile.StaticCredentials),
		Tls: &fmev1.EndpointTlsSettings{
			CaBundle:   transferProfile.TLS.CABundle,
			SkipVerify: transferProfile.TLS.SkipVerify,
		},
	}
}

func oidcConfigToProtobuf(oidc *OIDCConfig) *fmev1.OIDCConfig {
	if oidc == nil {
		return nil
	}
	return &fmev1.OIDCConfig{
		IssuerUrl:              oidc.IssuerURL,
		ClientId:               oidc.ClientID,
		RoleArn:                oidc.RoleARN,
		Scopes:                 oidc.Scopes,
		PersistSession:         oidc.PersistSession,
		CustomCaBundle:         oidc.CustomCABundle,
		SessionDurationSeconds: oidc.SessionDurationSeconds,
	}
}

func staticCredentialsToProtobuf(creds *StaticCredentials) *fmev1.StaticCredentials {
	if creds == nil {
		return nil
	}
	return &fmev1.StaticCredentials{
		AccessKeyId:     creds.AccessKeyID,
		SecretAccessKey: creds.SecretAccessKey,
		SessionToken:    creds.SessionToken,
	}
}

func HotFoldersFromProtobuf(hotFoldersList []*fmev1.UploadHotFolderSettings) (hotFolders []UploadHotFolderSettings) {
	for _, hotFolder := range hotFoldersList {
		var txs []HotFolderRemoteConfigurations
		for _, tx := range hotFolder.RemoteConfigurations {
			transferConfiguration := HotFolderRemoteConfigurations{
				RemoteConfigurationName: tx.RemoteConfigurationName,
				S3DestinationFolder:     tx.S3DestinationFolder,
			}
			txs = append(txs, transferConfiguration)
		}
		hotFolderFromProtobuf := UploadHotFolderSettings{
			Name:                 hotFolder.Name,
			Enabled:              hotFolder.Enabled,
			LocalSourceFolder:    hotFolder.LocalSourceFolder,
			RemoteConfigurations: txs,
			ForceInitialUpload:   hotFolder.ForceInitialUpload,
		}
		hotFolders = append(hotFolders, hotFolderFromProtobuf)
	}

	return hotFolders
}

func HotFoldersToProtobuf(hotFolders []UploadHotFolderSettings) []*fmev1.UploadHotFolderSettings {
	var out []*fmev1.UploadHotFolderSettings
	for _, hotFolder := range hotFolders {
		var txs []*fmev1.HotFolderTransferConfigurations
		for _, tx := range hotFolder.RemoteConfigurations {
			pbv := &fmev1.HotFolderTransferConfigurations{
				RemoteConfigurationName: tx.RemoteConfigurationName,
				S3DestinationFolder:     tx.S3DestinationFolder,
			}
			txs = append(txs, pbv)
		}
		pbv := &fmev1.UploadHotFolderSettings{
			Enabled:              hotFolder.Enabled,
			LocalSourceFolder:    hotFolder.LocalSourceFolder,
			Name:                 hotFolder.Name,
			RemoteConfigurations: txs,
			ForceInitialUpload:   hotFolder.ForceInitialUpload,
		}
		out = append(out, pbv)
	}

	return out
}
