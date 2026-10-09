import { describe, it, expect, beforeEach, vi } from 'vitest';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { TransferProfileFormComponent } from './transfer-profile-form.component';
import { AppState } from '@app/state';
import { MatExpansionModule } from '@angular/material/expansion';
import { MatDialog, MatDialogModule } from '@angular/material/dialog';
import { ReactiveFormsModule } from '@angular/forms';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatSlideToggleModule } from '@angular/material/slide-toggle';
import { MatTooltipModule } from '@angular/material/tooltip';
import { MatChipsModule } from '@angular/material/chips';
import { MatSnackBarModule } from '@angular/material/snack-bar';
import { StoreModule } from '@ngrx/store';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { provideMockStore } from '@ngrx/store/testing';
import { initialTestState } from '@state/test.state';
import { MatAutocompleteModule } from '@angular/material/autocomplete';
import { FmeClientService } from '@services/fme-client/fme-client.service';
import { of, Subject, throwError } from 'rxjs';
import { TransferProfile } from '@app/classes';
import { ValidateTransferProfileResponse } from '@gen/es/fme/v1/config_pb';
import { ProviderType, REDACTED_SECRET } from '@app/constants/providers.constants';
import { MatSelect } from '@angular/material/select';
import { By } from '@angular/platform-browser';

describe('TransferProfileFormComponent', () => {
    let component: TransferProfileFormComponent;
    let fixture: ComponentFixture<TransferProfileFormComponent>;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [
                MatExpansionModule,
                MatDialogModule,
                ReactiveFormsModule,
                MatInputModule,
                MatSelectModule,
                MatFormFieldModule,
                MatSlideToggleModule,
                MatTooltipModule,
                MatChipsModule,
                MatSnackBarModule,
                StoreModule,
                MatAutocompleteModule,
            ],
            providers: [
                MatDialog,
                MatBottomSheet,
                FmeClientService,
                provideMockStore<AppState>({initialState: initialTestState}),
            ],
        })
            .compileComponents();
    });

    beforeEach(() => {
        fixture = TestBed.createComponent(TransferProfileFormComponent);
        component = fixture.componentInstance;
        fixture.detectChanges();
    });

    it('should create', () => {
        expect(component).toBeTruthy();
    });

    it('should default auth method to aws-profile', () => {
        const authMethod = component.transferProfileForm.get('authMethod');
        expect(authMethod?.value).toBe('aws-profile');
    });

    it('should not require OIDC fields when auth method is aws-profile', () => {
        const issuerUrl = component.transferProfileForm.get('oidcIssuerUrl');
        const clientId = component.transferProfileForm.get('oidcClientId');
        const roleArn = component.transferProfileForm.get('oidcRoleArn');

        issuerUrl?.setValue('');
        clientId?.setValue('');
        roleArn?.setValue('');

        expect(issuerUrl?.valid).toBe(true);
        expect(clientId?.valid).toBe(true);
        expect(roleArn?.valid).toBe(true);
    });

    it('should require OIDC fields when auth method is oidc', () => {
        const authMethod = component.transferProfileForm.get('authMethod');
        authMethod?.setValue('oidc');

        const issuerUrl = component.transferProfileForm.get('oidcIssuerUrl');
        const clientId = component.transferProfileForm.get('oidcClientId');
        const roleArn = component.transferProfileForm.get('oidcRoleArn');

        issuerUrl?.setValue('');
        clientId?.setValue('');
        roleArn?.setValue('');

        expect(issuerUrl?.hasError('required')).toBe(true);
        expect(clientId?.hasError('required')).toBe(true);
        expect(roleArn?.hasError('required')).toBe(true);
    });

    it('should clear OIDC validation errors when switching back to aws-profile', () => {
        const authMethod = component.transferProfileForm.get('authMethod');
        authMethod?.setValue('oidc');

        const issuerUrl = component.transferProfileForm.get('oidcIssuerUrl');
        issuerUrl?.setValue('');
        expect(issuerUrl?.hasError('required')).toBe(true);

        authMethod?.setValue('aws-profile');
        expect(issuerUrl?.hasError('required')).toBe(false);
        expect(issuerUrl?.valid).toBe(true);
    });

    it('should have default scopes value', () => {
        const scopes = component.transferProfileForm.get('oidcScopes');
        expect(scopes?.value).toBe('openid, email, profile, offline_access');
    });

    it('should have persist session defaulted to false', () => {
        const persist = component.transferProfileForm.get('oidcPersistSession');
        expect(persist?.value).toBe(false);
    });

    it('should have session duration defaulted to 0', () => {
        const duration = component.transferProfileForm.get('oidcSessionDurationSeconds');
        expect(duration?.value).toBe(0);
    });

    it('should show redirect URI hint when auth method is oidc', () => {
        const authMethod = component.transferProfileForm.get('authMethod');
        authMethod?.setValue('oidc');
        fixture.detectChanges();

        const hint = fixture.nativeElement.querySelector('.redirect-uri-hint');
        expect(hint).toBeTruthy();
        expect(hint.textContent).toContain('127.0.0.1:9876/callback');
        expect(hint.textContent).toContain('127.0.0.1:9877/callback');
        expect(hint.textContent).toContain('127.0.0.1:9878/callback');
    });

    it('should not show redirect URI hint when auth method is aws-profile', () => {
        const authMethod = component.transferProfileForm.get('authMethod');
        authMethod?.setValue('aws-profile');
        fixture.detectChanges();

        const hint = fixture.nativeElement.querySelector('.redirect-uri-hint');
        expect(hint).toBeNull();
    });

    describe('getOriginalHint', () => {
        it('strips the appended extended-hint span', () => {
            const withExtended = 'Base hint.<span class="extended-hint"> extra</span>';
            expect(component.getOriginalHint(withExtended)).toBe('Base hint.');
        });

        it('returns the message unchanged when there is no extended hint', () => {
            expect(component.getOriginalHint('Just the base hint.')).toBe('Just the base hint.');
        });
    });

    describe('getBucketError', () => {
        it('returns empty when the bucket control has no errors', () => {
            component.transferProfileForm.controls['bucket'].setValue('valid-bucket');
            expect(component.getBucketError()).toBe('');
        });

        it('maps a bucketURI error to the leading-s3 message', () => {
            component.transferProfileForm.controls['bucket'].setErrors({bucketURI: true});
            expect(component.getBucketError()).toContain('s3://');
        });

        it('maps an invalidPrefix error to the prefix message', () => {
            component.transferProfileForm.controls['bucket'].setErrors({invalidPrefix: 'xn--'});
            expect(component.getBucketError()).toContain('xn--');
        });
    });

    describe('file-order chips', () => {
        it('adds a chip and clears the input', () => {
            component.transferProfileForm.controls['fileOrder'].setValue([]);
            const clear = vi.fn();
            component.addFileOrderChip({value: '.mov', chipInput: {clear}} as never);
            expect(component.transferProfileForm.controls['fileOrder'].value).toEqual(['.mov']);
            expect(clear).toHaveBeenCalled();
        });

        it('ignores an empty chip value', () => {
            component.transferProfileForm.controls['fileOrder'].setValue(['.mov']);
            component.addFileOrderChip({value: ''} as never);
            expect(component.transferProfileForm.controls['fileOrder'].value).toEqual(['.mov']);
        });

        it('removes a chip by extension', () => {
            component.transferProfileForm.controls['fileOrder'].setValue(['.mov', '.mxf']);
            component.removeFileOrderChip('.mov');
            expect(component.transferProfileForm.controls['fileOrder'].value).toEqual(['.mxf']);
        });

        it('reports invalid extensions in the file-order error string', () => {
            component.transferProfileForm.controls['fileOrder'].setValue(['not a valid ext!']);
            expect(component.getFileOrderErrors()).toContain('Invalid extensions');
        });
    });
});

describe('TransferProfileFormComponent S3-compatible providers', () => {
    let component: TransferProfileFormComponent;
    let fixture: ComponentFixture<TransferProfileFormComponent>;
    let validateTransferProfile: ReturnType<typeof vi.fn>;

    function query<T extends Element>(selector: string): T {
        return (fixture.nativeElement as HTMLElement).querySelector(selector) as T;
    }

    function endpointInput(): HTMLInputElement {
        return query<HTMLInputElement>('input[formControlName="endpoint"]');
    }

    function selectProvider(provider: ProviderType) {
        component.transferProfileForm.get('provider')?.setValue(provider);
        component.onProviderSelected(provider);
        fixture.detectChanges();
    }

    async function createInUpdateMode(profile: TransferProfile): Promise<void> {
        await configure();
        fixture = TestBed.createComponent(TransferProfileFormComponent);
        component = fixture.componentInstance;
        fixture.componentRef.setInput('mode', 'update');
        fixture.componentRef.setInput('transferProfile', profile);
        fixture.detectChanges();
        // setData() runs on the next tick
        await new Promise((resolve) => setTimeout(resolve, 0));
        fixture.detectChanges();
    }

    async function configure(): Promise<void> {
        await TestBed.configureTestingModule({
            imports: [
                MatExpansionModule,
                MatDialogModule,
                ReactiveFormsModule,
                MatInputModule,
                MatSelectModule,
                MatFormFieldModule,
                MatSlideToggleModule,
                MatTooltipModule,
                MatChipsModule,
                MatSnackBarModule,
                StoreModule,
                MatAutocompleteModule,
            ],
            providers: [
                MatDialog,
                MatBottomSheet,
                FmeClientService,
                provideMockStore<AppState>({initialState: initialTestState}),
            ],
        }).compileComponents();
        // Spy on the real service so every other member the form (and its child components) use keeps working.
        validateTransferProfile = vi.spyOn(TestBed.inject(FmeClientService), 'validateTransferProfile')
            .mockReturnValue(of({
                success: true,
                message: 'successfully listed bucket media',
            } as ValidateTransferProfileResponse)) as unknown as ReturnType<typeof vi.fn>;
    }

    function minioProfile(): TransferProfile {
        return TransferProfile.fromJson({
            name: 'minio-local',
            bucket: 'media',
            region: 'eu-west-1',
            profile: '',
            accelerated: false,
            fileOrder: [],
            enableMetadataFilter: true,
            storageClass: 'standard',
            paths: {local: '', remote: ''},
            chunkSize: 25,
            threads: 10,
            maxAge: '',
            filter: '',
            checksums: {enabled: false, algorithm: 'none'},
            autoTuning: true,
            endpoint: 'http://nas.local:9000',
            authMethod: 'static-keys',
            provider: 'minio',
            pathStyle: false,
            accessKeyId: 'minioadmin',
            secretAccessKey: 'minioadmin',
            tlsSkipVerify: true,
        });
    }

    describe('add mode', () => {
        beforeEach(async () => {
            await configure();
            fixture = TestBed.createComponent(TransferProfileFormComponent);
            component = fixture.componentInstance;
            fixture.detectChanges();
        });

        it('defaults to Amazon S3 with an optional endpoint', () => {
            expect(component.transferProfileForm.get('provider')?.value).toBe('aws');
            expect(component.isAws).toBe(true);
            expect(component.transferProfileForm.get('endpoint')?.hasError('required')).toBe(false);
            expect(component.transferProfileForm.get('pathStyle')?.value).toBe(false);
        });

        it('prefills the MinIO preset and requires a valid endpoint URL', () => {
            const form = component.transferProfileForm;
            form.get('provider')?.setValue('minio');
            component.onProviderSelected('minio');
            fixture.detectChanges();

            // The example endpoint is only a placeholder, never a value that would pass validation.
            expect(form.get('endpoint')?.value).toBe('');
            expect(endpointInput().placeholder).toBe('https://minio.example.com:9000');
            expect(form.get('pathStyle')?.value).toBe(true);
            expect(form.get('region')?.value).toBe('us-east-1');
            expect(component.isAws).toBe(false);

            form.get('endpoint')?.setValue('');
            expect(form.get('endpoint')?.hasError('required')).toBe(true);
            form.get('endpoint')?.setValue('nas.local:9000');
            expect(form.get('endpoint')?.hasError('invalidEndpoint')).toBe(true);
            form.get('endpoint')?.setValue('http://nas.local:9000');
            expect(form.get('endpoint')?.valid).toBe(true);
        });

        it('keeps a user-typed endpoint when switching provider but replaces a template', () => {
            const form = component.transferProfileForm;
            form.get('provider')?.setValue('minio');
            component.onProviderSelected('minio');
            form.get('endpoint')?.setValue('http://nas.local:9000');

            form.get('provider')?.setValue('wasabi');
            component.onProviderSelected('wasabi');
            expect(form.get('endpoint')?.value).toBe('http://nas.local:9000');

            form.get('endpoint')?.setValue('');
            form.get('provider')?.setValue('cloudflare-r2');
            component.onProviderSelected('cloudflare-r2');
            fixture.detectChanges();
            // {account_id} is only known to the user: shown as the placeholder, not filled in.
            expect(form.get('endpoint')?.value).toBe('');
            expect(endpointInput().placeholder).toBe('https://{account_id}.r2.cloudflarestorage.com');
            // The region still held the previous preset's default, so it follows the new preset.
            expect(form.get('region')?.value).toBe('auto');

            form.get('region')?.setValue('eu-central-1');
            form.get('provider')?.setValue('backblaze-b2');
            component.onProviderSelected('backblaze-b2');
            expect(form.get('region')?.value).toBe('eu-central-1');
            expect(form.get('endpoint')?.value).toBe('https://s3.eu-central-1.backblazeb2.com');
        });

        it('prefills the Hetzner preset with a location-based endpoint and region', () => {
            const form = component.transferProfileForm;
            form.get('provider')?.setValue('hetzner');
            component.onProviderSelected('hetzner');

            expect(form.get('endpoint')?.value).toBe('https://fsn1.your-objectstorage.com');
            expect(form.get('endpoint')?.valid).toBe(true);
            expect(form.get('region')?.value).toBe('fsn1');
            expect(form.get('pathStyle')?.value).toBe(false);
            expect(component.preset.supportsStorageClass).toBe(false);
            expect(component.isAws).toBe(false);
        });

        it('switches OIDC to access keys when leaving Amazon S3', () => {
            const form = component.transferProfileForm;
            form.get('authMethod')?.setValue('oidc');
            form.get('provider')?.setValue('minio');
            expect(form.get('authMethod')?.value).toBe('static-keys');
        });

        it('requires both access keys only for the static-keys auth method', () => {
            const form = component.transferProfileForm;
            form.get('authMethod')?.setValue('static-keys');
            expect(form.get('accessKeyId')?.hasError('required')).toBe(true);
            expect(form.get('secretAccessKey')?.hasError('required')).toBe(true);

            form.get('accessKeyId')?.setValue('AKIA');
            form.get('secretAccessKey')?.setValue('secret');
            expect(form.get('accessKeyId')?.valid).toBe(true);
            expect(form.get('secretAccessKey')?.valid).toBe(true);

            form.get('authMethod')?.setValue('aws-profile');
            form.get('accessKeyId')?.setValue('');
            expect(form.get('accessKeyId')?.hasError('required')).toBe(false);
        });

        it('relaxes AWS-only validation for other providers', () => {
            const form = component.transferProfileForm;
            form.get('provider')?.setValue('minio');
            component.onProviderSelected('minio');
            form.get('bucket')?.setValue('xn--bucket');
            expect(form.get('bucket')?.valid).toBe(true);
            form.get('storageClass')?.setValue('COLD');
            expect(form.get('storageClass')?.valid).toBe(true);

            form.get('provider')?.setValue('aws');
            component.onProviderSelected('aws');
            // Free-text storage classes mean nothing to Amazon S3; the bucket name is the user's to fix.
            expect(form.get('storageClass')?.value).toBe('standard');
            expect(form.get('storageClass')?.valid).toBe(true);
            expect(form.get('bucket')?.hasError('invalidBucket')).toBe(true);
        });

        it('hides Transfer Acceleration for non-Amazon providers', () => {
            const text = () => (fixture.nativeElement as HTMLElement).textContent ?? '';
            expect(text()).toContain('Transfer Acceleration');
            component.transferProfileForm.get('provider')?.setValue('minio');
            fixture.detectChanges();
            expect(text()).not.toContain('Transfer Acceleration');
            expect(text()).toContain('path-style');
        });

        it('tests the connection with the current form values', () => {
            const form = component.transferProfileForm;
            form.get('provider')?.setValue('minio');
            component.onProviderSelected('minio');
            form.patchValue({
                bucket: 'media',
                endpoint: 'http://nas.local:9000',
                authMethod: 'static-keys',
                accessKeyId: 'minioadmin',
                secretAccessKey: 'minioadmin',
            });
            expect(component.canTestConnection()).toBe(true);

            component.testConnection()();

            expect(validateTransferProfile).toHaveBeenCalledTimes(1);
            const sent = validateTransferProfile.mock.calls[0][0];
            expect(sent.bucket).toBe('media');
            expect(sent.endpoint).toBe('http://nas.local:9000');
            expect(sent.pathStyle).toBe(true);
            expect(sent.staticCredentials?.accessKeyId).toBe('minioadmin');
            expect(component.testState).toBe('ok');
            expect(component.testMessage).toContain('media');
        });

        it('reports a failed connection test inline', () => {
            validateTransferProfile.mockReturnValueOnce(of({success: false, message: 'SignatureDoesNotMatch'}));
            component.transferProfileForm.patchValue({bucket: 'media', region: 'us-east-1'});
            component.testConnection()();
            expect(component.testState).toBe('error');
            expect(component.testMessage).toBe('SignatureDoesNotMatch');

            validateTransferProfile.mockReturnValueOnce(throwError(() => new Error('daemon unreachable')));
            component.testConnection()();
            expect(component.testState).toBe('error');
            expect(component.testMessage).toBe('daemon unreachable');
        });

        it('does not test while required connection fields are invalid', () => {
            component.transferProfileForm.get('provider')?.setValue('minio');
            component.transferProfileForm.get('endpoint')?.setValue('');
            expect(component.canTestConnection()).toBe(false);
            component.testConnection()();
            expect(validateTransferProfile).not.toHaveBeenCalled();
        });
    });

    describe('update mode', () => {
        beforeEach(async () => {
            await createInUpdateMode(minioProfile());
        });

        it('loads a saved non-Amazon profile without applying the preset over it', () => {
            const form = component.transferProfileForm;
            expect(form.get('provider')?.value).toBe('minio');
            expect(form.get('endpoint')?.value).toBe('http://nas.local:9000');
            expect(form.get('pathStyle')?.value).toBe(false);
            expect(form.get('region')?.value).toBe('eu-west-1');
            expect(form.get('authMethod')?.value).toBe('static-keys');
            expect(form.get('accessKeyId')?.value).toBe('minioadmin');
            expect(form.get('tlsSkipVerify')?.value).toBe(true);
            expect(form.valid).toBe(true);
        });
    });

    describe('add mode: provider-aware UI', () => {
        beforeEach(async () => {
            await configure();
            fixture = TestBed.createComponent(TransferProfileFormComponent);
            component = fixture.componentInstance;
            component.regions = ['us-east-1', 'eu-west-1'];
            fixture.detectChanges();
        });

        function authOption(value: string) {
            const select = fixture.debugElement.query(By.css('mat-select[formControlName="authMethod"]'))
                .componentInstance as MatSelect;
            return select.options.find((option) => option.value === value);
        }

        it('shows password inputs with accessible visibility toggles', () => {
            component.transferProfileForm.get('authMethod')?.setValue('static-keys');
            fixture.detectChanges();

            const secret = query<HTMLInputElement>('.secret-input');
            const token = query<HTMLInputElement>('.session-token-input');
            const secretToggle = query<HTMLButtonElement>('button.secret-toggle');
            const tokenToggle = query<HTMLButtonElement>('button.session-token-toggle');
            expect(secret.type).toBe('password');
            expect(token.type).toBe('password');
            expect(secretToggle.type).toBe('button');
            expect(secretToggle.getAttribute('aria-label')).toBe('Show secret access key');
            expect(tokenToggle.getAttribute('aria-label')).toBe('Show session token');

            secretToggle.click();
            tokenToggle.click();
            fixture.detectChanges();
            expect(secret.type).toBe('text');
            expect(token.type).toBe('text');
            expect(secretToggle.getAttribute('aria-label')).toBe('Hide secret access key');

            // Changing the auth method hides the secrets again.
            component.transferProfileForm.get('authMethod')?.setValue('aws-profile');
            component.transferProfileForm.get('authMethod')?.setValue('static-keys');
            fixture.detectChanges();
            expect(query<HTMLInputElement>('.secret-input').type).toBe('password');
            expect(query<HTMLInputElement>('.session-token-input').type).toBe('password');
        });

        it('clears typed secrets when switching away from access keys', () => {
            const form = component.transferProfileForm;
            form.get('authMethod')?.setValue('static-keys');
            component.secretInput.setValue('s3cr3t');
            component.sessionTokenInput.setValue('tok');
            expect(form.get('secretAccessKey')?.value).toBe('s3cr3t');
            expect(form.get('sessionToken')?.value).toBe('tok');

            form.get('authMethod')?.setValue('aws-profile');
            expect(component.secretInput.value).toBe('');
            expect(component.sessionTokenInput.value).toBe('');
            expect(form.get('secretAccessKey')?.value).toBe('');
            expect(form.get('sessionToken')?.value).toBe('');
        });

        it('renders the skip-verify warning with the warning style and icon', () => {
            component.transferProfileForm.get('tlsSkipVerify')?.setValue(true);
            fixture.detectChanges();
            const warning = query<HTMLElement>('.toggle-note.warning-hint');
            expect(warning.textContent).toContain('Insecure');
            expect(warning.querySelector('mat-icon')?.textContent).toContain('warning');
        });

        it('warns about plain http endpoints only for access keys', () => {
            selectProvider('minio');
            component.transferProfileForm.get('endpoint')?.setValue('http://nas.local:9000');
            fixture.detectChanges();
            expect(query('.endpoint-warning')).toBeNull();

            component.transferProfileForm.get('authMethod')?.setValue('static-keys');
            fixture.detectChanges();
            expect(query('.endpoint-warning')?.textContent).toContain('not encrypted');

            component.transferProfileForm.get('endpoint')?.setValue('https://nas.local:9000');
            fixture.detectChanges();
            expect(query('.endpoint-warning')).toBeNull();
        });

        it('offers OIDC only for Amazon S3', () => {
            expect(authOption('oidc')?.disabled).toBe(false);
            selectProvider('minio');
            expect(authOption('oidc')?.disabled).toBe(true);
            expect(authOption('static-keys')?.disabled).toBe(false);
        });

        it('uses a region select for Amazon S3 and a text input otherwise', () => {
            expect(query('mat-select[formControlName="region"]')).toBeTruthy();
            expect(query('input[formControlName="region"]')).toBeNull();
            expect((fixture.nativeElement as HTMLElement).textContent).toContain('The AWS Region that your S3 bucket is located in.');

            selectProvider('cloudflare-r2');
            expect(query('mat-select[formControlName="region"]')).toBeNull();
            expect(query<HTMLInputElement>('input[formControlName="region"]').value).toBe('auto');
        });

        it('hides the storage class for providers without storage classes', () => {
            expect(query('mat-select[formControlName="storageClass"]')).toBeTruthy();
            selectProvider('minio');
            expect(query('input[formControlName="storageClass"]')).toBeTruthy();
            selectProvider('hetzner');
            expect(query('[formControlName="storageClass"]')).toBeNull();
            // A hidden field must not block saving.
            component.transferProfileForm.get('storageClass')?.setValue('');
            expect(component.transferProfileForm.get('storageClass')?.valid).toBe(true);
        });

        it('labels the endpoint field per provider', () => {
            const label = () => endpointInput().closest('mat-form-field')?.querySelector('mat-label')?.textContent?.trim();
            expect(label()).toBe('S3 Access Point / VPC Endpoint');
            selectProvider('wasabi');
            expect(label()).toBe('Endpoint URL');
        });

        it('shows a spinner and disables the button while testing', () => {
            validateTransferProfile.mockReturnValue(new Subject<ValidateTransferProfileResponse>());
            component.transferProfileForm.patchValue({bucket: 'media', region: 'us-east-1'});
            fixture.detectChanges();
            const button = query<HTMLButtonElement>('.test-connection fme-button button');
            expect(button.disabled).toBe(false);

            component.testConnection()();
            fixture.detectChanges();
            expect(component.testState).toBe('testing');
            expect(query('.test-connection-result mat-spinner')).toBeTruthy();
            expect(query('.test-connection-result')?.getAttribute('aria-live')).toBe('polite');
            expect(button.disabled).toBe(true);
        });

        it('resets AWS-only values when switching from another provider to Amazon S3', () => {
            const form = component.transferProfileForm;
            selectProvider('minio');
            form.patchValue({region: 'garage', endpoint: 'http://nas.local:9000', storageClass: 'COLD'});

            selectProvider('aws');
            expect(form.get('region')?.value).toBe('');
            expect(form.get('region')?.hasError('required')).toBe(true);
            // The error blocks saving, so it is shown right away.
            expect(form.get('region')?.touched).toBe(true);
            expect(form.get('endpoint')?.value).toBe('');
            expect(form.get('storageClass')?.value).toBe('standard');
            expect(form.get('pathStyle')?.value).toBe(false);
        });

        it('keeps an AWS Region when switching from another provider to Amazon S3', () => {
            const form = component.transferProfileForm;
            selectProvider('minio');
            form.get('region')?.setValue('eu-west-1');
            selectProvider('aws');
            expect(form.get('region')?.value).toBe('eu-west-1');
        });

        it('replaces AWS values with the new preset when leaving Amazon S3', () => {
            const form = component.transferProfileForm;
            form.patchValue({region: 'eu-west-1', endpoint: 'https://bucket.vpce-1.s3.eu-west-1.vpce.amazonaws.com', accelerated: true});

            selectProvider('cloudflare-r2');
            expect(form.get('region')?.value).toBe('auto');
            expect(form.get('endpoint')?.value).toBe('');
            expect(form.get('accelerated')?.value).toBe(false);
        });

        it('fills in the region for B2 and Hetzner endpoints and follows the provider', () => {
            const form = component.transferProfileForm;
            selectProvider('backblaze-b2');
            expect(form.get('region')?.value).toBe('us-west-004');
            expect(form.get('endpoint')?.value).toBe('https://s3.us-west-004.backblazeb2.com');

            // Both values were prefilled by B2, so Hetzner replaces them.
            selectProvider('hetzner');
            expect(form.get('region')?.value).toBe('fsn1');
            expect(form.get('endpoint')?.value).toBe('https://fsn1.your-objectstorage.com');

            // Spaces signs with us-east-1, so {region} in its hostname is left for the user.
            selectProvider('digitalocean-spaces');
            expect(form.get('endpoint')?.value).toBe('');
            expect(endpointInput().placeholder).toBe('https://{region}.digitaloceanspaces.com');
        });

        it('clears the test result when a connection field changes', () => {
            component.transferProfileForm.patchValue({bucket: 'media', region: 'us-east-1'});
            component.testConnection()();
            expect(component.testState).toBe('ok');

            component.transferProfileForm.get('bucket')?.setValue('other-bucket');
            expect(component.testState).toBe('idle');
            expect(component.testMessage).toBe('');

            component.testConnection()();
            component.transferProfileForm.get('tlsSkipVerify')?.setValue(true);
            expect(component.testState).toBe('idle');
        });

        it('ignores a late response after the provider changed', () => {
            const response = new Subject<ValidateTransferProfileResponse>();
            validateTransferProfile.mockReturnValue(response);
            component.transferProfileForm.patchValue({bucket: 'media', region: 'us-east-1'});
            component.testConnection()();
            expect(response.observed).toBe(true);

            selectProvider('wasabi');
            expect(response.observed).toBe(false);
            response.next({success: true, message: 'stale'} as ValidateTransferProfileResponse);
            expect(component.testState).toBe('idle');
            expect(component.testMessage).toBe('');
        });

        it('cancels a running test on destroy', () => {
            const response = new Subject<ValidateTransferProfileResponse>();
            validateTransferProfile.mockReturnValue(response);
            component.transferProfileForm.patchValue({bucket: 'media', region: 'us-east-1'});
            component.testConnection()();
            expect(response.observed).toBe(true);

            fixture.destroy();
            expect(response.observed).toBe(false);
        });
    });

    describe('update mode with an OIDC profile for another provider', () => {
        beforeEach(async () => {
            const profile = minioProfile();
            profile.authMethod = 'oidc';
            await createInUpdateMode(profile);
        });

        it('loads it with access keys, since OIDC is Amazon-only', () => {
            expect(component.transferProfileForm.get('provider')?.value).toBe('minio');
            expect(component.transferProfileForm.get('authMethod')?.value).toBe('static-keys');
        });
    });

    describe('update mode with saved secrets', () => {
        beforeEach(async () => {
            const profile = minioProfile();
            profile.secretAccessKey = REDACTED_SECRET;
            profile.sessionToken = REDACTED_SECRET;
            await createInUpdateMode(profile);
        });

        function allInputValues(): string[] {
            return Array.from((fixture.nativeElement as HTMLElement).querySelectorAll('input')).map((input) => input.value);
        }

        it('shows the secret inputs empty with a "saved" hint', () => {
            expect(component.secretInput.value).toBe('');
            expect(query<HTMLInputElement>('.secret-input').value).toBe('');
            expect(query<HTMLInputElement>('.session-token-input').value).toBe('');
            expect(allInputValues()).not.toContain(REDACTED_SECRET);
            const text = (fixture.nativeElement as HTMLElement).textContent ?? '';
            expect(text).toContain('Saved — leave empty to keep the current secret');
            expect(text).toContain('Saved — leave empty to keep the current session token');
            expect(component.transferProfileForm.valid).toBe(true);
        });

        it('sends the placeholder for empty inputs while the access key ID is unchanged', () => {
            const raw = component.transferProfileForm.getRawValue();
            expect(raw.secretAccessKey).toBe(REDACTED_SECRET);
            expect(raw.sessionToken).toBe(REDACTED_SECRET);

            component.testConnection()();
            const sent = validateTransferProfile.mock.calls[0][0];
            expect(sent.staticCredentials?.secretAccessKey).toBe(REDACTED_SECRET);
            expect(sent.staticCredentials?.sessionToken).toBe(REDACTED_SECRET);
        });

        it('sends a newly typed secret instead of the placeholder', () => {
            component.secretInput.setValue('new-secret');
            expect(component.transferProfileForm.getRawValue().secretAccessKey).toBe('new-secret');
        });

        it('requires the secret again when the access key ID changes', () => {
            const form = component.transferProfileForm;
            form.get('accessKeyId')?.setValue('other-key');
            fixture.detectChanges();

            expect(form.getRawValue().secretAccessKey).toBe('');
            expect(form.getRawValue().sessionToken).toBe('');
            expect(component.secretInput.hasError('required')).toBe(true);
            expect(form.valid).toBe(false);
            expect((fixture.nativeElement as HTMLElement).textContent).toContain('Enter the secret access key for the new access key ID.');

            component.secretInput.setValue('other-secret');
            expect(form.getRawValue().secretAccessKey).toBe('other-secret');
            expect(component.secretInput.valid).toBe(true);
            expect(form.valid).toBe(true);

            // Switching back to the saved key ID keeps the stored secret again.
            component.secretInput.setValue('');
            form.get('accessKeyId')?.setValue('minioadmin');
            expect(form.getRawValue().secretAccessKey).toBe(REDACTED_SECRET);
        });

        it('removes the saved session token on request', () => {
            const toggle = query('.clear-session-token');
            expect(toggle).toBeTruthy();
            component.clearSessionToken.setValue(true);
            expect(component.transferProfileForm.getRawValue().sessionToken).toBe('');
            expect(component.transferProfileForm.getRawValue().secretAccessKey).toBe(REDACTED_SECRET);
        });
    });
});
