// Barrel kamus bagian aplikasi. Satu berkas per area; setiap berkas exporters
// `en` + `id` dengan kunci yang sama persis (tipe `id` memaksa itu).
// Spread (bukan Object.assign) supaya kunci tetap literal: salah ketik kunci
// jadi error TypeScript, bukan teks kosong.
import { en as accEn, id as accId } from './account';
import { en as selEn, id as selId } from './seller';
import { en as shlEn, id as shlId } from './shell';
import { en as walEn, id as walId } from './wallet';
import { en as publicPagesEn, id as publicPagesId } from './public-pages';

export const appEn = { ...accEn, ...selEn, ...shlEn, ...walEn, ...publicPagesEn };
export const appId = { ...accId, ...selId, ...shlId, ...walId, ...publicPagesId };
