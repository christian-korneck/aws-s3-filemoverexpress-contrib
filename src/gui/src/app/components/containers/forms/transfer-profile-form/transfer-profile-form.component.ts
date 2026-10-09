import { CdkDrag, CdkDragDrop, CdkDropList, moveItemInArray } from '@angular/cdk/drag-drop';
import { NgClass } from '@angular/common';
import { AfterContentInit, AfterViewInit, Component, ElementRef, inject, input, OnDestroy, OnInit, output, ViewChild, ChangeDetectionStrategy } from '@angular/core';
import { AbstractControl, FormControl, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { MatAutocomplete, MatAutocompleteTrigger } from '@angular/material/autocomplete';
import { MatIconButton } from '@angular/material/button';
import { MatChipGrid, MatChipInput, MatChipInputEvent, MatChipRow } from '@angular/material/chips';
import { MatIcon } from '@angular/material/icon';
import { MatError, MatFormField, MatHint, MatInput, MatLabel, MatSuffix } from '@angular/material/input';
import { MatProgressSpinner } from '@angular/material/progress-spinner';
import { MatOption, MatSelect } from '@angular/material/select';
import { MatSlideToggle } from '@angular/material/slide-toggle';
import { BrowseButtonComponent } from '@primitives/forms/browse-button/browse-button.component';
import {
    autotuningFieldsIsIntegerValidator,
    autotuningFieldsRequiredValidator,
    bucketValidator,
    chunksizeMinValidator,
    endpointUrlValidator,
    fileExtensionRegExp,
    handleStreamError,
    oidcIssuerUrlValidator,
    s3ArnRgx,
    threadsMinValidator,
    TransferProfile,
} from '@app/classes';
import { ButtonComponent } from '@primitives/buttons/button/button.component';
import { HintPopoverService } from '@services/hint-popover/hint-popover.service';
import { formErrorMessages } from '@app/constants/common.constants';
import {
    DEFAULT_PROVIDER,
    presetEndpoint,
    presetFor,
    ProviderPreset,
    providerPresets,
    ProviderType,
    REDACTED_SECRET,
} from '@app/constants/providers.constants';
import { ObjectSortPipe } from '@app/pipes/object-sort.pipe';
import { isPackagedApp } from '@app/utils/utils';
import { MetadataEvent } from '@events/core';
import { FmeClientService } from '@services/fme-client/fme-client.service';
import { RegionsService } from '@services/regions/regions.service';
import { merge, Subscription } from 'rxjs';
import {
    awsStorageClassValidators,
    checksumAlgorithms,
    createTransferProfileForm,
    separatorKeysCodes,
    storageClasses,
} from './transfer-profile-form.constants';
import { EditorMode, StorageClass, TestConnectionState, TransferProfileForm } from './transfer-profile-form.interfaces';
import { WailsService } from '@services/wails/wails.service';

@Component({
    selector: 'fme-transfer-profile-form',
    templateUrl: './transfer-profile-form.component.html',
    styleUrls: ['./transfer-profile-form.component.scss'],
    changeDetection: ChangeDetectionStrategy.Eager,
    imports: [
        ReactiveFormsModule,
        MatFormField,
        MatLabel,
        MatHint,
        MatError,
        MatInput,
        MatSuffix,
        BrowseButtonComponent,
        MatSelect,
        MatOption,
        MatAutocompleteTrigger,
        MatAutocomplete,
        MatChipGrid,
        CdkDropList,
        MatChipRow,
        NgClass,
        CdkDrag,
        MatIcon,
        MatChipInput,
        MatSlideToggle,
        MatProgressSpinner,
        MatIconButton,
        ButtonComponent,
        ObjectSortPipe,
    ],
})
export class TransferProfileFormComponent implements OnInit, OnDestroy, AfterViewInit, AfterContentInit {
    private regionsService = inject(RegionsService);
    private fmeClientService = inject(FmeClientService);
    private hintPopover = inject(HintPopoverService);
    private wails = inject(WailsService);

    tutorialMode = input<boolean>(false);
    transferProfile = input<TransferProfile | null>();
    mode = input<EditorMode>('add');
    transferProfileEdited = output<FormGroup<TransferProfileForm>>();
    // non-null assertion operator (!) added for ViewChildren since they can only be initialized in AfterViewInit
    @ViewChild('fileOrderChipList') fileOrderChipList!: MatChipGrid;
    @ViewChild('bucketHint') bucketHint!: ElementRef;
    transferProfileForm: FormGroup = new FormGroup({});
    // Left-nav section (matches the Settings page). Panels stay mounted; nav toggles them.
    activeTab: 'connection' | 'authentication' | 'performance' = 'connection';
    daemonOS = '';
    errorMessages = formErrorMessages;
    regions: string[];
    storageClasses: StorageClass[] = storageClasses;
    checksumAlgorithms = checksumAlgorithms;
    awsProfiles: string[] = [];
    protected readonly formErrorMessages = formErrorMessages;
    checksumSelector = new FormControl<string>('none');
    providerPresets: ProviderPreset[] = providerPresets;
    // "Test connection" state shown next to the button in the Connection tab.
    testState: TestConnectionState = 'idle';
    testMessage = '';
    showSecret = false;
    showSessionToken = false;
    // The secret inputs live outside the form group: the group holds the value sent to the daemon
    // (REDACTED_SECRET to keep a saved secret) while the inputs only ever show what the user typed.
    secretInput = new FormControl<string>('', {nonNullable: true});
    sessionTokenInput = new FormControl<string>('', {nonNullable: true});
    clearSessionToken = new FormControl<boolean>(false, {nonNullable: true});

    private subscriptions: Subscription[] = [];
    private testSubscription?: Subscription;
    // Which secrets the daemon holds for the loaded profile; the daemon only returns REDACTED_SECRET.
    private savedCredentials = {accessKeyId: '', secret: false, sessionToken: false};
    // Tracks the preset before/after the last provider change so prefilled values can be replaced
    // when the user switches provider without clobbering values they typed themselves.
    private previousPreset: ProviderPreset = presetFor(DEFAULT_PROVIDER);
    private currentPreset: ProviderPreset = presetFor(DEFAULT_PROVIDER);
    readonly separatorKeysCodes = separatorKeysCodes;

    constructor() {
        this.regions = this.regionsService.getRegions();

        this.regionsService.regions$.pipe(handleStreamError({retryCount: 3})).subscribe({
            next: (regions) => {
                this.regions = regions;
            },
            error: (error) => {
                this.fmeClientService.processStreamError(error);
            },
        });

        this.subscriptions.push(this.fmeClientService.metadata.subscribe(this.processMetadata.bind(this)));
    }

    ngOnInit() {
        this.transferProfileForm = this.setupFormGroup();
    }

    ngOnDestroy() {
        this.subscriptions.map((sub) => sub.unsubscribe());
        this.subscriptions = [];
        this.testSubscription?.unsubscribe();
    }

    ngAfterViewInit() {
        const bucketControl = this.transferProfileForm.get('bucket');
        if (bucketControl) {
            const bucketSubscription = bucketControl.valueChanges.subscribe(
                () => {
                    // In the tabbed layout the bucket hint only exists while the Connection
                    // tab is rendered; guard against it being absent (e.g. another tab active).
                    if (!this.bucketHint) {
                        return;
                    }
                    const bucketHintString = (this.bucketHint.nativeElement as Element).innerHTML;
                    (this.bucketHint.nativeElement as Element).innerHTML = this.getOriginalHint(bucketHintString);
                },
            );
            this.subscriptions.push(bucketSubscription);
        } else {
            console.debug('Unable to get Remote Configuration bucket form control');
        }
    }

    ngAfterContentInit() {
        const ctrlEnabled = this.transferProfileForm.get('checksums.enabled');
        const ctrlValue = this.transferProfileForm.get('checksums.algorithm');
        if (ctrlEnabled && ctrlValue) {
            if (ctrlEnabled.value === true) {
                this.checksumSelector.setValue(ctrlValue.value);
            } else {
                this.checksumSelector.setValue('none');
            }
        }

        this.checksumSelector.valueChanges.subscribe((newValue) => {
            if (ctrlEnabled && ctrlValue) {
                if (!newValue || newValue === 'none') {
                    ctrlEnabled.setValue(false);
                    ctrlValue.setValue('none');
                } else {
                    ctrlEnabled.setValue(true);
                    ctrlValue.setValue(newValue);
                }
            }
        });
    }

    private setData() {
        const data = Object.assign({}, this.transferProfile());
        // Stored secrets come back as REDACTED_SECRET. Remember which exist so that an empty input keeps
        // them, and never put the placeholder into an input.
        this.savedCredentials = {
            accessKeyId: data.accessKeyId ?? '',
            secret: data.secretAccessKey === REDACTED_SECRET,
            sessionToken: data.sessionToken === REDACTED_SECRET,
        };
        this.secretInput.setValue(this.savedCredentials.secret ? '' : data.secretAccessKey ?? '', {emitEvent: false});
        this.sessionTokenInput.setValue(this.savedCredentials.sessionToken ? '' : data.sessionToken ?? '', {emitEvent: false});
        this.transferProfileForm.setValue(data);
        this.syncSecrets(this.transferProfileForm);
        if (!this.transferProfileForm.controls.storageClass.value) {
            this.transferProfileForm.patchValue({storageClass: 'standard'});
        }
        this.transferProfileForm.markAllAsTouched();
        this.secretInput.markAsTouched();
    }

    setupFormGroup(): FormGroup<TransferProfileForm> {
        const form = createTransferProfileForm(this.storageClasses, this.checksumAlgorithms);

        // OIDC conditional validation: require fields when auth method is OIDC
        const authMethodControl = form.get('authMethod');
        if (authMethodControl) {
            const oidcFieldNames = ['oidcIssuerUrl',
                'oidcClientId',
                'oidcRoleArn'] as const;
            const issuerUrlAsyncValidator = oidcIssuerUrlValidator(this.wails);
            const authMethodSubscription = authMethodControl.valueChanges.subscribe((value) => {
                for (const fieldName of oidcFieldNames) {
                    const control = form.get(fieldName);
                    if (control) {
                        if (value === 'oidc') {
                            control.addValidators(Validators.required);
                            if (fieldName === 'oidcIssuerUrl') {
                                control.addAsyncValidators(issuerUrlAsyncValidator);
                            }
                        } else {
                            control.removeValidators(Validators.required);
                            if (fieldName === 'oidcIssuerUrl') {
                                control.removeAsyncValidators(issuerUrlAsyncValidator);
                            }
                        }
                        control.updateValueAndValidity({onlySelf: true});
                        // Switching auth method shouldn't paint empty fields red on its own.
                        // Reset the touched/dirty state so the "required" errors only surface
                        // once the user actually edits a field (or attempts to save).
                        control.markAsUntouched();
                        control.markAsPristine();
                    }
                }
                // Static access keys: both halves of the key pair are required.
                for (const fieldName of ['accessKeyId', 'secretAccessKey'] as const) {
                    const control = form.get(fieldName);
                    if (control) {
                        if (value === 'static-keys') {
                            control.addValidators(Validators.required);
                        } else {
                            control.removeValidators(Validators.required);
                        }
                        control.updateValueAndValidity({onlySelf: true});
                        control.markAsUntouched();
                        control.markAsPristine();
                    }
                }
                this.showSecret = false;
                this.showSessionToken = false;
                if (value !== 'static-keys') {
                    // Don't keep typed secrets around for an auth method that doesn't send them.
                    this.secretInput.setValue('', {emitEvent: false});
                    this.sessionTokenInput.setValue('', {emitEvent: false});
                    this.clearSessionToken.setValue(false, {emitEvent: false});
                }
                this.secretInput.markAsUntouched();
                this.syncSecrets(form);
            });
            this.subscriptions.push(authMethodSubscription);
        }

        // The secret input mirrors the errors of the secretAccessKey control it feeds.
        this.secretInput.setValidators(() => form.controls.secretAccessKey.errors);
        const secretsSubscription = merge(
            this.secretInput.valueChanges,
            this.sessionTokenInput.valueChanges,
            this.clearSessionToken.valueChanges,
            form.controls.accessKeyId.valueChanges,
        ).subscribe(() => this.syncSecrets(form));
        this.subscriptions.push(secretsSubscription);

        form.get('chunkSize')?.addValidators([
            autotuningFieldsRequiredValidator,
            autotuningFieldsIsIntegerValidator,
            chunksizeMinValidator,
        ]);

        form.get('threads')?.addValidators([
            autotuningFieldsRequiredValidator,
            autotuningFieldsIsIntegerValidator,
            threadsMinValidator,
        ]);

        const autotuningControl = form.get('autoTuning');
        const autotuningThreadsControl = form.get('threads');
        const autotuningChunkSizeControl = form.get('chunkSize');
        if (autotuningControl && autotuningThreadsControl && autotuningChunkSizeControl) {
            const autotuningSubscription = autotuningControl.valueChanges.subscribe(
                () => {
                    autotuningThreadsControl.updateValueAndValidity({onlySelf: true});
                    autotuningChunkSizeControl.updateValueAndValidity({onlySelf: true});
                },
            );
            this.subscriptions.push(autotuningSubscription);
        } else {
            console.debug('Unable to get Remote Configuration autotuning form controls');
        }

        form.get('bucket')?.addValidators(bucketValidator);

        const acceleratedControl = form.get('accelerated');
        const bucketControl = form.get('bucket');
        if (acceleratedControl && bucketControl) {
            const acceleratedSubscription = acceleratedControl.valueChanges.subscribe(
                () => {
                    bucketControl.updateValueAndValidity({onlySelf: true});
                },
            );
            this.subscriptions.push(acceleratedSubscription);
        } else {
            console.debug('Unable to get Remote Configuration accelerated and bucket form controls');
        }

        // Provider-dependent validation. Runs on every value change (including setValue in update
        // mode) so validators always match the provider; prefilling happens only on user selection.
        this.syncProviderValidators(form, form.controls.provider.value);
        const providerSubscription = form.controls.provider.valueChanges.subscribe((value) => {
            this.previousPreset = this.currentPreset;
            this.currentPreset = presetFor(value);
            this.syncProviderValidators(form, value);
        });
        this.subscriptions.push(providerSubscription);

        // A test result only describes the values it was run with.
        const connectionFields = [
            'provider',
            'bucket',
            'region',
            'endpoint',
            'pathStyle',
            'authMethod',
            'profile',
            'accessKeyId',
            'secretAccessKey',
            'sessionToken',
            'tlsCaBundle',
            'tlsSkipVerify',
        ] as const;
        const connectionSubscription = merge(...connectionFields.map((name) => form.controls[name].valueChanges))
            .subscribe(() => this.resetTestResult());
        this.subscriptions.push(connectionSubscription);

        this.transferProfileEdited.emit(form);

        form.valueChanges.subscribe(
            () => {
                this.transferProfileEdited.emit(form);
            },
        );

        if (this.mode() === 'update') {
            if (!this.transferProfile()) {
                console.error('transferProfile attribute must be set in update mode');
            } else {
                form.get('name')?.disable();
                setTimeout(() => {
                    this.setData(); // Runs on next tick to wait for form controls to be registered
                });
            }
        }

        return form;
    }

    /** The preset for the currently selected provider (Amazon S3 until the form is initialised). */
    get preset(): ProviderPreset {
        return presetFor(this.transferProfileForm.get('provider')?.value);
    }

    get isAws(): boolean {
        return this.preset.isAws;
    }

    /** Example or template endpoint shown in the empty endpoint input. */
    get endpointPlaceholder(): string {
        return this.preset.endpointExample || this.preset.endpointTemplate;
    }

    /** True when access keys are used against a plain http:// endpoint. */
    get insecureEndpoint(): boolean {
        const form = this.transferProfileForm;
        return form.get('authMethod')?.value === 'static-keys'
            && /^http:\/\//i.test((form.get('endpoint')?.value ?? '').trim());
    }

    /** True while an empty secret input keeps the secret the daemon already stores. */
    get hasSavedSecret(): boolean {
        return this.savedCredentials.secret && this.accessKeyIdUnchanged;
    }

    /** True while an empty session token input keeps the token the daemon already stores. */
    get hasSavedSessionToken(): boolean {
        return this.savedCredentials.sessionToken && this.accessKeyIdUnchanged;
    }

    /** True when a stored secret can no longer be kept because the access key ID was changed. */
    get secretMustBeReentered(): boolean {
        return this.savedCredentials.secret && !this.accessKeyIdUnchanged;
    }

    private get accessKeyIdUnchanged(): boolean {
        return this.transferProfileForm.get('accessKeyId')?.value === this.savedCredentials.accessKeyId;
    }

    /**
     * Applies the chosen provider's preset. Bound to the select's (selectionChange) rather than
     * valueChanges so that loading a saved profile in update mode never overwrites its values.
     * Values the user typed for another S3-compatible provider are kept; values that cannot be valid
     * for the new provider (AWS Regions, endpoints and storage classes across the AWS boundary) or
     * that the previous preset filled in are replaced.
     */
    onProviderSelected(provider: ProviderType) {
        const preset = presetFor(provider);
        const previous = this.previousPreset;
        const form = this.transferProfileForm as FormGroup<TransferProfileForm>;
        const {region, endpoint, storageClass} = form.controls;
        const previousRegion = region.value;
        const changed: AbstractControl[] = [];
        const update = (control: FormControl<string>, value: string) => {
            if (control.value !== value) {
                control.setValue(value);
                changed.push(control);
            }
        };

        if (preset.isAws) {
            if (!this.regions.includes(region.value)) {
                update(region, preset.defaultRegion);
            }
            if (!previous.isAws) {
                update(endpoint, '');
            }
            if (!this.storageClasses.some((sc) => sc.key === storageClass.value)) {
                update(storageClass, 'standard');
            }
        } else {
            if (previous.isAws || !region.value || region.value === previous.defaultRegion) {
                update(region, preset.defaultRegion);
            }
            const prefilledByPrevious = endpoint.value === presetEndpoint(previous, previousRegion)
                || endpoint.value === presetEndpoint(previous, previous.defaultRegion);
            if (previous.isAws || !endpoint.value || prefilledByPrevious) {
                update(endpoint, presetEndpoint(preset, region.value));
            }
        }
        form.controls.pathStyle.setValue(preset.pathStyle);
        if (!preset.isAws) {
            form.controls.accelerated.setValue(false);
        }
        // Errors caused by the switch block saving, so show them right away.
        for (const control of changed) {
            if (control.invalid) {
                control.markAsTouched();
            }
        }
    }

    private syncProviderValidators(form: FormGroup<TransferProfileForm>, provider: string) {
        const preset = presetFor(provider);
        const endpoint = form.controls.endpoint;
        const storageClass = form.controls.storageClass;
        const authMethod = form.controls.authMethod;

        if (preset.isAws) {
            endpoint.setValidators([]);
            storageClass.setValidators(awsStorageClassValidators(this.storageClasses));
        } else {
            endpoint.setValidators([Validators.required, endpointUrlValidator]);
            // The field is hidden for providers without storage classes, so it must not block saving.
            storageClass.setValidators(preset.supportsStorageClass ? [Validators.required] : []);
        }
        if (!preset.supportsOidc && authMethod.value === 'oidc') {
            // OIDC depends on AWS STS; fall back to the auth method every S3-compatible store supports.
            authMethod.setValue('static-keys');
        }
        endpoint.updateValueAndValidity({onlySelf: true});
        storageClass.updateValueAndValidity({onlySelf: true});
        form.controls.bucket.updateValueAndValidity({onlySelf: true});
    }

    /**
     * Writes the values sent to the daemon into the secretAccessKey and sessionToken controls: what
     * the user typed, otherwise REDACTED_SECRET to keep a stored secret while the access key ID is
     * unchanged, otherwise an empty string.
     */
    private syncSecrets(form: FormGroup<TransferProfileForm>) {
        const staticKeys = form.controls.authMethod.value === 'static-keys';
        const keyIdUnchanged = form.controls.accessKeyId.value === this.savedCredentials.accessKeyId;
        const keepSecret = this.savedCredentials.secret && keyIdUnchanged;
        const keepSessionToken = this.savedCredentials.sessionToken && keyIdUnchanged && !this.clearSessionToken.value;
        const secret = this.secretInput.value || (keepSecret ? REDACTED_SECRET : '');
        const sessionToken = this.sessionTokenInput.value || (keepSessionToken ? REDACTED_SECRET : '');
        form.patchValue({
            secretAccessKey: staticKeys ? secret : '',
            sessionToken: staticKeys ? sessionToken : '',
        });
        this.secretInput.updateValueAndValidity({emitEvent: false});
        if (staticKeys && this.savedCredentials.secret && !keyIdUnchanged) {
            // The stored secret belongs to the old key ID; point the user at the field to re-enter it.
            this.secretInput.markAsTouched();
        }
    }

    private resetTestResult() {
        this.testSubscription?.unsubscribe();
        this.testSubscription = undefined;
        this.testState = 'idle';
        this.testMessage = '';
    }

    /** True when the fields the daemon needs to reach the bucket are valid and no test is running. */
    canTestConnection(): boolean {
        if (this.testState === 'testing') {
            return false;
        }
        const form = this.transferProfileForm;
        const required: string[] = [
            'bucket',
            'region',
            'endpoint',
        ];
        if (form.get('authMethod')?.value === 'static-keys') {
            required.push('accessKeyId', 'secretAccessKey');
        }
        return required.every((name) => form.get(name)?.valid !== false);
    }

    /**
     * Returns the click handler for the "Test connection" button (see ButtonComponent.onClick). The
     * current, possibly unsaved, form values are sent to the daemon, which lists the bucket once.
     */
    testConnection(): () => void {
        return () => {
            if (!this.canTestConnection()) {
                return;
            }
            this.resetTestResult();
            this.testState = 'testing';
            const profile = TransferProfile.fromJson(this.transferProfileForm.getRawValue());
            this.testSubscription = this.fmeClientService.validateTransferProfile(TransferProfile.jsonToProtobuf(profile)).subscribe({
                next: (response) => {
                    this.testState = response.success ? 'ok' : 'error';
                    this.testMessage = response.message;
                },
                error: (error: unknown) => {
                    this.testState = 'error';
                    this.testMessage = error instanceof Error ? error.message : String(error);
                },
            });
        };
    }

    getOriginalHint(hintMessage: string) {
        const extendedHintIndex = (hintMessage.indexOf('<span class="extended-hint">'));
        if (extendedHintIndex !== -1) {
            hintMessage = hintMessage.substring(0, extendedHintIndex);
        }
        return hintMessage;
    }

    addFileOrderChip(evt: MatChipInputEvent) {
        if (evt.value == '') {
            return;
        }
        const newChips = this.transferProfileForm.controls['fileOrder'].value || [];
        newChips.push(evt.value);
        evt.chipInput!.clear();
        this.transferProfileForm.controls['fileOrder'].setValue(newChips);
        this.fileOrderChipList.errorState = !!this.transferProfileForm.controls['fileOrder'].errors;
    }

    getFileOrderErrors() {
        this.fileOrderChipList.errorState = !!this.transferProfileForm.controls['fileOrder'].errors;
        const extensionList = this.transferProfileForm.controls['fileOrder'].value.toString().split(',');
        let invalidExtensions = '';
        extensionList.forEach((extension: string) => {
            if (!fileExtensionRegExp.test(extension)) {
                invalidExtensions += invalidExtensions ? `, ${extension}` : `${extension}`;
            }
        });
        return invalidExtensions ? `Invalid extensions: ${invalidExtensions}` : '';
    }

    removeFileOrderChip(ext: string) {
        const newChips = this.transferProfileForm.controls['fileOrder'].value.filter((x: string) => x !== ext);
        this.transferProfileForm.controls['fileOrder'].setValue(newChips);
        this.fileOrderChipList.errorState = !!this.transferProfileForm.controls['fileOrder'].errors;
    }

    chipListMovement(event: CdkDragDrop<string[]>) {
        moveItemInArray(this.transferProfileForm.controls['fileOrder'].value, event.previousIndex, event.currentIndex);
    }

    onBucketPaste(event: ClipboardEvent) {
        if (event.clipboardData) {
            this.setBucketValueAndHint(event.clipboardData.getData('text').trim(), event);
        }
    }

    onBucketBlur(event: FocusEvent) {
        this.setBucketValueAndHint(this.transferProfileForm.controls['bucket'].value, event);
    }

    setBucketValueAndHint(fieldValue: string, event: Event) {
        if (fieldValue?.length) {
            const match = fieldValue.match(s3ArnRgx)?.groups;

            if (match) {
                event.stopImmediatePropagation();
                event.preventDefault();

                let hintMessage = (this.bucketHint.nativeElement as Element).innerHTML;

                hintMessage = `${this.getOriginalHint(hintMessage)} <span class="extended-hint">`;
                if (this.tutorialMode()) {
                    hintMessage += '<br> ';
                }
                hintMessage += event instanceof ClipboardEvent
                    ? `Pasted <code>${fieldValue}</code>.`
                    : `Removed <code>s3://</code> from <code>${fieldValue}</code>.`;

                this.transferProfileForm.controls['bucket'].setValue(match['bucket']);
                (this.bucketHint.nativeElement as Element).innerHTML = hintMessage + '</span>';
            }
        }
    }

    getBucketError(): string {
        const error = this.transferProfileForm.controls['bucket'].errors;
        if (!error) {
            return '';
        }
        const errorStrings = {
            acceleratedWithPeriods: 'Buckets used with Amazon S3 Transfer Acceleration can\'t have dots (.) in their names',
            bucketURI: 'Enter bucket name without leading <code>s3://</code>',
            invalidPrefix: `Bucket name cannot start with <code>${error['invalidPrefix']}</code>`,
            invalidSuffix: `Bucket name cannot end with <code>${error['invalidSuffix']}</code>`,
            hasAdjacentPeriods: 'Bucket name cannot have adjacent periods',
            ipAddressFormat: 'Bucket name cannot be formatted as an IP address',
        };
        for (const [
            errorCode, errorMessage,
        ] of Object.entries(errorStrings)) {
            if (errorCode in error) {
                return errorMessage;
            }
        }
        return 'Bucket name format is invalid or there are invalid characters';
    }

    openExternalLink(event: Event, url: string) {
        event.preventDefault();

        if (isPackagedApp()) {
            this.wails.externalLink(url).subscribe();
        }
    }

    toggleHint(event: MouseEvent, message: string) {
        event.stopPropagation();
        event.preventDefault();

        this.hintPopover.open(event.currentTarget as HTMLElement, message);
    }

    private processMetadata(metadata: MetadataEvent) {
        this.daemonOS = metadata.daemonOS;
        this.awsProfiles = metadata.awsProfiles;
    }
}
