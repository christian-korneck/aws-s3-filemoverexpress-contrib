import { describe, it, expect } from 'vitest';
import {
    bucketValidator,
    endpointUrlValidator,
    fileOrderListValidator,
    maxActiveChecksumsValidator,
    noSpacesValidator,
    s3ArnRgx,
} from './form-validators';
import { FormGroup, UntypedFormControl } from '@angular/forms';

describe('fileOrderListValidator', () => {
    it('should return null with empty input', () => {
        const control = new UntypedFormControl('input');
        control.setValue([]);
        expect(fileOrderListValidator(control)).toBeNull();

        control.setValue(null);
        expect(fileOrderListValidator(control)).toBeNull();
    });

    it('should return null with valid input', () => {
        const control = new UntypedFormControl('input');
        control.setValue([
            '.exr', '.mov',
        ]);
        expect(fileOrderListValidator(control)).toBeNull();
    });

    it('should return error with invalid input', () => {
        const control = new UntypedFormControl('input');
        control.setValue([
            'exr', 'mov',
        ]);
        expect(fileOrderListValidator(control)).toEqual({invalidFileOrder: true});
    });
});

describe('maxActiveChecksumsValidator', () => {
    const validator = maxActiveChecksumsValidator(10);

    it('should return null with valid input', () => {
        const control = new UntypedFormControl('input');
        control.setValue(10);

        expect(validator(control)).toBeNull();
    });

    it('should return error with invalid input', () => {
        const control = new UntypedFormControl('input');

        control.setValue(12);
        expect(validator(control)).toEqual({invalidChecksum: true});

        control.setValue('string');
        expect(validator(control)).toEqual({invalidChecksum: true});
    });
});

describe('noSpacesValidator', () => {
    it('should return null with valid input', () => {
        const control = new UntypedFormControl('input');
        control.setValue('helloworld');
        expect(noSpacesValidator(control)).toBeNull();
    });

    it('should return error with invalid input', () => {
        const control = new UntypedFormControl('input');
        control.setValue('hello world');
        expect(noSpacesValidator(control)).toEqual({hasSpaces: true});
    });
});

describe('s3ArnRgx', () => {
    it('captures the bucket from an s3:// URI', () => {
        expect('s3://my-bucket'.match(s3ArnRgx)?.groups?.['bucket']).toBe('my-bucket');
    });

    it('captures the bucket from a full S3 ARN (issue #27)', () => {
        expect('arn:aws:s3:::example-bucket'.match(s3ArnRgx)?.groups?.['bucket']).toBe('example-bucket');
    });

    it('captures the bucket from an ARN with a trailing key/path', () => {
        expect('arn:aws:s3:::my-bucket/some/key'.match(s3ArnRgx)?.groups?.['bucket']).toBe('my-bucket');
    });

    it('does not match a bare bucket name (nothing to strip)', () => {
        expect('example-bucket'.match(s3ArnRgx)).toBeNull();
    });
});

describe('endpointUrlValidator', () => {
    it('accepts empty values and full http(s) URLs', () => {
        const control = new UntypedFormControl('');
        expect(endpointUrlValidator(control)).toBeNull();
        control.setValue('https://minio.example.com:9000');
        expect(endpointUrlValidator(control)).toBeNull();
        control.setValue('http://127.0.0.1:9000/');
        expect(endpointUrlValidator(control)).toBeNull();
    });

    it('rejects values without a scheme or with a non-http scheme', () => {
        const control = new UntypedFormControl('minio.example.com:9000');
        expect(endpointUrlValidator(control)).toEqual({invalidEndpoint: true});
        control.setValue('ftp://minio.example.com');
        expect(endpointUrlValidator(control)).toEqual({invalidEndpoint: true});
        control.setValue('https://');
        expect(endpointUrlValidator(control)).toEqual({invalidEndpoint: true});
        control.setValue('http://');
        expect(endpointUrlValidator(control)).toEqual({invalidEndpoint: true});
    });

    it('rejects unreplaced template placeholders', () => {
        const control = new UntypedFormControl('https://{account_id}.r2.cloudflarestorage.com');
        expect(endpointUrlValidator(control)).toEqual({invalidEndpoint: true});
        control.setValue('https://s3.{region}.backblazeb2.com');
        expect(endpointUrlValidator(control)).toEqual({invalidEndpoint: true});
    });

    it('rejects surrounding whitespace', () => {
        const control = new UntypedFormControl(' https://minio.example.com');
        expect(endpointUrlValidator(control)).toEqual({invalidEndpoint: true});
        control.setValue('https://minio.example.com\t');
        expect(endpointUrlValidator(control)).toEqual({invalidEndpoint: true});
    });

    it('accepts IPv6 hosts', () => {
        expect(endpointUrlValidator(new UntypedFormControl('http://[::1]:9000'))).toBeNull();
    });
});

describe('bucketValidator provider awareness', () => {
    function formWithProvider(provider: string, bucket: string): UntypedFormControl {
        const group = new FormGroup({
            provider: new UntypedFormControl(provider),
            accelerated: new UntypedFormControl(false),
            bucket: new UntypedFormControl(bucket),
        });
        return group.get('bucket') as UntypedFormControl;
    }

    it('applies AWS naming rules for Amazon S3', () => {
        expect(bucketValidator(formWithProvider('aws', 'xn--bucket'))).toEqual({invalidBucket: true, invalidPrefix: 'xn--'});
        expect(bucketValidator(formWithProvider('aws', 'my..bucket'))).toEqual({invalidBucket: true, hasAdjacentPeriods: true});
        expect(bucketValidator(formWithProvider('aws', 'my-bucket'))).toBeNull();
    });

    it('only rejects obvious mistakes for other providers', () => {
        expect(bucketValidator(formWithProvider('minio', 'My_Bucket'))).toBeNull();
        expect(bucketValidator(formWithProvider('minio', 'xn--bucket'))).toBeNull();
        expect(bucketValidator(formWithProvider('minio', 's3://bucket'))).toEqual({invalidBucket: true, bucketURI: true});
        expect(bucketValidator(formWithProvider('minio', 'my bucket'))).toEqual({invalidBucket: true});
    });

    it('ignores Transfer Acceleration for other providers', () => {
        const bucket = formWithProvider('minio', 'my.bucket');
        bucket.parent?.get('accelerated')?.setValue(true);
        expect(bucketValidator(bucket)).toBeNull();

        const awsBucket = formWithProvider('aws', 'my.bucket');
        awsBucket.parent?.get('accelerated')?.setValue(true);
        expect(bucketValidator(awsBucket)).toEqual({invalidBucket: true, acceleratedWithPeriods: true});
    });

    it('applies AWS naming rules to a control without a parent form', () => {
        expect(bucketValidator(new UntypedFormControl('my..bucket'))).toEqual({invalidBucket: true, hasAdjacentPeriods: true});
        expect(bucketValidator(new UntypedFormControl('my-bucket'))).toBeNull();
    });
});
