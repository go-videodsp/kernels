package main

import "math/bits"

// A synthetic Annex B H.264 stream whose P pictures isolate one decoder
// stage. Written from ITU-T H.264 (V16) (06/2026): the NAL unit syntax of 7.3.1,
// the SPS of 7.3.2.1.1, the PPS of 7.3.2.2, the slice header of 7.3.3, the
// prediction weight table of 7.3.3.2, the slice data of 7.3.4 and the
// macroblock layer of 7.3.5, with the Exp-Golomb codes of 9.1.
//
// The stream is Baseline (CAVLC, one reference frame, no B pictures), 8-bit
// 4:2:0. The first picture is an IDR picture of I_PCM macroblocks (raw
// samples, so any texture is representable with no transform). Every P
// picture is one slice of P_L0_16x16 macroblocks with the same motion vector
// and coded_block_pattern 0, so the decoder does motion compensation and
// nothing else per macroblock: no residual, deblocking disabled.
//
// With a uniform motion field only the first macroblock has a non-zero
// mvd: the median predictor of 8.4.1.3 equals the vector everywhere else
// (a single available neighbour is copied, two equal ones win the median,
// and an unavailable C is replaced by D).

// bitWriter writes MSB-first bits.
type bitWriter struct {
	buf  []byte
	nbit int // bits used in the last byte, 0..7
}

func (w *bitWriter) bit(b uint32) {
	if w.nbit == 0 {
		w.buf = append(w.buf, 0)
	}
	w.buf[len(w.buf)-1] |= byte(b&1) << (7 - w.nbit)
	w.nbit = (w.nbit + 1) & 7
}

// u writes the n low bits of v, MSB first: u(n) of clause 7.2.
func (w *bitWriter) u(n int, v uint32) {
	for i := n - 1; i >= 0; i-- {
		w.bit(v >> i)
	}
}

// ue is ue(v), Equation 9-1: codeNum + 1 in binary, preceded by as many
// zeros as it has bits after the first.
func (w *bitWriter) ue(v uint32) {
	n := bits.Len32(v + 1)
	w.u(n-1, 0)
	w.u(n, v+1)
}

// se is se(v), Equation 9-3: positive v maps to 2v - 1, the others to -2v.
func (w *bitWriter) se(v int) {
	if v > 0 {
		w.ue(uint32(2*v - 1))
		return
	}
	w.ue(uint32(-2 * v))
}

// trailing is rbsp_trailing_bits (7.3.2.11): a one, then zeros to the byte.
func (w *bitWriter) trailing() {
	w.bit(1)
	for w.nbit != 0 {
		w.bit(0)
	}
}

// align is pcm_alignment_zero_bit (7.3.5).
func (w *bitWriter) align() {
	for w.nbit != 0 {
		w.bit(0)
	}
}

// nal frames an RBSP as an Annex B NAL unit: start code, header byte, and the
// payload with emulation prevention (7.4.1: a 0x03 after two zeros when the
// next byte is 0..3).
func nal(refIDC, unitType int, rbsp []byte) []byte {
	out := []byte{0, 0, 0, 1, byte(refIDC<<5 | unitType)}
	zeros := 0
	for _, b := range rbsp {
		if zeros >= 2 && b <= 3 {
			out = append(out, 3)
			zeros = 0
		}
		out = append(out, b)
		if b == 0 {
			zeros++
		} else {
			zeros = 0
		}
	}
	return out
}

// weights is an explicit luma weight for P pictures (8.4.2.3.2); the chroma
// weights are left at their default.
type weights struct {
	denom, weight, offset int
}

// params describes a stream.
type params struct {
	wMB, hMB int      // picture size in macroblocks
	frames   int      // P pictures after the IDR picture
	mvx, mvy int      // motion vector in quarter luma samples
	weight   *weights // nil: no weighted prediction
}

func sps(p params) []byte {
	var w bitWriter
	w.u(8, 66) // profile_idc: Baseline
	w.u(8, 0)  // constraint flags, reserved
	w.u(8, 51) // level_idc
	w.ue(0)    // seq_parameter_set_id
	w.ue(0)    // log2_max_frame_num_minus4: frame_num is 4 bits
	w.ue(2)    // pic_order_cnt_type 2: output order is decoding order
	w.ue(1)    // max_num_ref_frames
	w.bit(0)   // gaps_in_frame_num_value_allowed_flag
	w.ue(uint32(p.wMB - 1))
	w.ue(uint32(p.hMB - 1))
	w.bit(1) // frame_mbs_only_flag
	w.bit(1) // direct_8x8_inference_flag
	w.bit(0) // frame_cropping_flag
	w.bit(0) // vui_parameters_present_flag
	w.trailing()
	return nal(3, 7, w.buf)
}

func pps(p params) []byte {
	var w bitWriter
	w.ue(0)  // pic_parameter_set_id
	w.ue(0)  // seq_parameter_set_id
	w.bit(0) // entropy_coding_mode_flag: CAVLC
	w.bit(0) // bottom_field_pic_order_in_frame_present_flag
	w.ue(0)  // num_slice_groups_minus1
	w.ue(0)  // num_ref_idx_l0_default_active_minus1
	w.ue(0)  // num_ref_idx_l1_default_active_minus1
	if p.weight != nil {
		w.bit(1) // weighted_pred_flag
	} else {
		w.bit(0)
	}
	w.u(2, 0) // weighted_bipred_idc
	w.se(0)   // pic_init_qp_minus26
	w.se(0)   // pic_init_qs_minus26
	w.se(0)   // chroma_qp_index_offset
	w.bit(1)  // deblocking_filter_control_present_flag
	w.bit(0)  // constrained_intra_pred_flag
	w.bit(0)  // redundant_pic_cnt_present_flag
	w.trailing()
	return nal(3, 8, w.buf)
}

// idr is an IDR picture of I_PCM macroblocks, the samples drawn from a fixed
// pseudo-random sequence (any texture works: the kernels are data independent).
func idr(p params) []byte {
	var w bitWriter
	w.ue(0)   // first_mb_in_slice
	w.ue(7)   // slice_type: I, all slices of the picture
	w.ue(0)   // pic_parameter_set_id
	w.u(4, 0) // frame_num
	w.ue(0)   // idr_pic_id
	w.bit(0)  // no_output_of_prior_pics_flag
	w.bit(0)  // long_term_reference_flag
	w.se(0)   // slice_qp_delta
	w.ue(1)   // disable_deblocking_filter_idc
	seed := uint32(12345)
	for i := 0; i < p.wMB*p.hMB; i++ {
		w.ue(25) // mb_type I_PCM
		w.align()
		for k := 0; k < 384; k++ {
			seed = seed*1664525 + 1013904223
			// Smooth-ish texture: a slow ramp plus noise, never constant.
			w.u(8, uint32(k*3&0x7f)+(seed>>24)&0x3f+32)
		}
	}
	w.trailing()
	return nal(3, 5, w.buf)
}

// pic is P picture number n (1-based).
func pic(p params, n int) []byte {
	var w bitWriter
	w.ue(0)              // first_mb_in_slice
	w.ue(5)              // slice_type: P, all slices of the picture
	w.ue(0)              // pic_parameter_set_id
	w.u(4, uint32(n%16)) // frame_num
	w.bit(0)             // num_ref_idx_active_override_flag
	w.bit(0)             // ref_pic_list_modification_flag_l0
	if p.weight != nil {
		w.ue(uint32(p.weight.denom)) // luma_log2_weight_denom
		w.ue(0)                      // chroma_log2_weight_denom
		w.bit(1)                     // luma_weight_l0_flag
		w.se(p.weight.weight)
		w.se(p.weight.offset)
		w.bit(0) // chroma_weight_l0_flag
	}
	w.bit(0) // adaptive_ref_pic_marking_mode_flag: sliding window
	w.se(0)  // slice_qp_delta
	w.ue(1)  // disable_deblocking_filter_idc
	for i := 0; i < p.wMB*p.hMB; i++ {
		w.ue(0) // mb_skip_run
		w.ue(0) // mb_type P_L0_16x16
		if i == 0 {
			w.se(p.mvx) // mvd_l0: the predictor is zero for the first macroblock
			w.se(p.mvy)
		} else {
			w.se(0)
			w.se(0)
		}
		w.ue(0) // coded_block_pattern 0 (Table 9-4, inter, codeNum 0)
	}
	w.trailing()
	return nal(2, 1, w.buf)
}

// stream is the whole Annex B byte stream.
func stream(p params) []byte {
	out := append(sps(p), pps(p)...)
	out = append(out, idr(p)...)
	for n := 1; n <= p.frames; n++ {
		out = append(out, pic(p, n)...)
	}
	return out
}
