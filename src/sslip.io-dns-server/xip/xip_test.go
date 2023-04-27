package xip_test

import (
	"encoding/binary"
	"math/rand"
	"net"
	"strings"
	"xip/xip"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/net/dns/dnsmessage"
)

var _ = Describe("Xip", func() {
	var (
		err error
	)
	rand.Seed(GinkgoRandomSeed()) // Set to ginkgo's seed so that it's different each test & we can reproduce failures if necessary

	Describe("CNAMEResources()", func() {
		It("returns nil by default", func() {
			randomDomain := random8ByteString() + ".com."
			cname := xip.CNAMEResource(randomDomain)
			Expect(cname).To(BeNil())
		})
		When("querying one of sslip.io's DKIM CNAME's", func() {
			It("returns the CNAME", func() {
				cname := xip.CNAMEResource("protonmail._domainkey.SSlip.Io.")
				Expect(cname.CNAME.String()).To(MatchRegexp("^protonmail\\.domainkey.*.domains\\.proton\\.ch\\.$"))
			})
		})
		When("a domain has been customized but has no CNAMEs", func() {
			It("returns nil", func() {
				customizedDomain := random8ByteString() + ".com."
				xip.Customizations[customizedDomain] = xip.DomainCustomization{}
				cname := xip.CNAMEResource(customizedDomain)
				Expect(cname).To(BeNil())
				delete(xip.Customizations, customizedDomain)
			})
		})
		When("a domain has been customized with CNAMES", func() {
			It("returns CNAME resources", func() {
				customizedDomain := random8ByteString() + ".com."
				xip.Customizations[strings.ToLower(customizedDomain)] = xip.DomainCustomization{
					CNAME: dnsmessage.CNAMEResource{
						CNAME: dnsmessage.Name{
							// google.com.
							Length: 11,
							Data: [255]byte{
								103, 111, 111, 103, 108, 101, 46, 99, 111, 109, 46,
							},
						},
					},
				}
				cname := xip.CNAMEResource(customizedDomain)
				Expect(cname.CNAME.String()).To(Equal("google.com."))
				delete(xip.Customizations, customizedDomain) // clean-up
			})
		})
	})

	Describe("MXResources()", func() {
		It("returns the MX resource", func() {
			randomDomain := random8ByteString() + ".com."
			mx := xip.MXResources(randomDomain)
			mxHostName := dnsmessage.MustNewName(randomDomain)
			Expect(len(mx)).To(Equal(1))
			Expect(mx[0].MX).To(Equal(mxHostName))
		})
		When("sslip.io is the domain being queried", func() {
			It("returns sslip.io's custom MX records", func() {
				mx := xip.MXResources("sslIP.iO.")
				Expect(len(mx)).To(Equal(2))
				Expect(mx[0].MX.Data).To(Equal(xip.Customizations["sslip.io."].MX[0].MX.Data))
			})
		})
	})

	Describe("NSResources()", func() {
		When("we use the default nameservers", func() {
			var x, _ = xip.NewXip("file:///", []string{"ns-aws.sslip.io.", "ns-azure.sslip.io.", "ns-gce.sslip.io."}, []string{}, map[string][]string{})
			It("returns the name servers", func() {
				randomDomain := random8ByteString() + ".com."
				ns := x.NSResources(randomDomain)
				Expect(len(ns)).To(Equal(3))
				Expect(ns[0].NS.String()).To(Equal("ns-aws.sslip.io."))
				Expect(ns[1].NS.String()).To(Equal("ns-azure.sslip.io."))
				Expect(ns[2].NS.String()).To(Equal("ns-gce.sslip.io."))
			})
			When(`the domain name contains "_acme-challenge."`, func() {
				When("the domain name has an embedded IP", func() {
					It(`returns an array of one NS record pointing to the domain name _sans_ "acme-challenge."`, func() {
						randomDomain := "192.168.0.1." + random8ByteString() + ".com."
						ns := x.NSResources("_acme-challenge." + randomDomain)
						Expect(len(ns)).To(Equal(1))
						Expect(ns[0].NS.String()).To(Equal(randomDomain))
						aResources := xip.NameToA(randomDomain)
						Expect(len(aResources)).To(Equal(1))
						Expect(err).ToNot(HaveOccurred())
						Expect(aResources[0].A).To(Equal([4]byte{192, 168, 0, 1}))
					})
				})
				When("the domain name does not have an embedded IP", func() {
					It("returns the default trinity of nameservers", func() {
						randomDomain := "_acme-challenge." + random8ByteString() + ".com."
						ns := x.NSResources(randomDomain)
						Expect(len(ns)).To(Equal(3))
					})
				})
			})
		})
		When("we override the default nameservers", func() {
			var x, _ = xip.NewXip("file:///", []string{"mickey", "minn.ie.", "goo.fy"}, []string{}, map[string][]string{})
			It("returns the configured servers", func() {
				randomDomain := random8ByteString() + ".com."
				ns := x.NSResources(randomDomain)
				Expect(len(ns)).To(Equal(3))
				Expect(ns[0].NS.String()).To(Equal("mickey."))
				Expect(ns[1].NS.String()).To(Equal("minn.ie."))
				Expect(ns[2].NS.String()).To(Equal("goo.fy."))
			})

		})
	})

	Describe("SOAResource()", func() {
		It("returns the SOA resource for the domain in question", func() {
			randomDomain := random8ByteString() + ".com."
			randomDomainName := dnsmessage.MustNewName(randomDomain)
			soa := xip.SOAResource(randomDomainName)
			Expect(soa.NS.Data).To(Equal(randomDomainName.Data))
		})
	})

	Describe("TXTResources()", func() {
		var x xip.Xip
		It("returns an empty array for a random domain", func() {
			randomDomain := random8ByteString() + ".com."
			txts, err := x.TXTResources(randomDomain, nil)
			Expect(err).To(Not(HaveOccurred()))
			Expect(len(txts)).To(Equal(0))
		})
		When("queried for the sslip.io domain", func() {
			It("returns mail-related TXT resources for the sslip.io domain", func() {
				domain := "ssLip.iO."
				txts, err := x.TXTResources(domain, nil)
				Expect(err).To(Not(HaveOccurred()))
				Expect(len(txts)).To(Equal(2))
				Expect(txts[0].TXT[0]).To(MatchRegexp("protonmail-verification="))
				Expect(txts[1].TXT[0]).To(MatchRegexp("v=spf1"))
			})
		})
		When("a random domain has been customized w/out any TXT defaults", func() { // Unnecessary, but confirms Golang's behavior for me, a doubting Thomas
			customizedDomain := random8ByteString() + ".com."
			xip.Customizations[customizedDomain] = xip.DomainCustomization{}
			It("returns no TXT resources", func() {
				txts, err := x.TXTResources(customizedDomain, nil)
				Expect(err).To(Not(HaveOccurred()))
				Expect(len(txts)).To(Equal(0))
			})
			delete(xip.Customizations, customizedDomain) // clean-up
		})
		When(`the domain "ip.sslip.io" is queried`, func() {
			It("returns the IP address of the querier", func() {
				txts, err := x.TXTResources("ip.sslip.io.", net.IP{1, 1, 1, 1})
				Expect(err).To(Not(HaveOccurred()))
				Expect(len(txts)).To(Equal(1))
				Expect(txts[0].TXT[0]).To(MatchRegexp("^1.1.1.1$"))
			})
		})
		When(`a customized domain without a TXT entry is queried`, func() {
			It("returns no records (and doesn't panic, either)", func() {
				txts, err := x.TXTResources("ns.sslip.io.", nil)
				Expect(err).To(Not(HaveOccurred()))
				Expect(len(txts)).To(Equal(0))
			})
		})
	})

	Describe("NameToA()", func() {
		xip.Customizations["custom.record."] = xip.DomainCustomization{A: []dnsmessage.AResource{
			{A: [4]byte{78, 46, 204, 247}},
		}}
		DescribeTable("when it succeeds",
			func(fqdn string, expectedA dnsmessage.AResource) {
				ipv4Answers := xip.NameToA(fqdn)
				Expect(len(ipv4Answers)).To(Equal(1))
				Expect(ipv4Answers[0]).To(Equal(expectedA))
			},
			Entry("custom record", "CusTom.RecOrd.", dnsmessage.AResource{A: [4]byte{78, 46, 204, 247}}),
			// dots
			Entry("loopback", "127.0.0.1", dnsmessage.AResource{A: [4]byte{127, 0, 0, 1}}),
			Entry("255 with domain", "255.254.253.252.com", dnsmessage.AResource{A: [4]byte{255, 254, 253, 252}}),
			Entry(`"This" network, pre-and-post`, "nono.io.0.1.2.3.ssLIp.IO", dnsmessage.AResource{A: [4]byte{0, 1, 2, 3}}),
			Entry("private network, two IPs, grabs the leftmost", "nono.io.172.16.0.30.172.31.255.255.sslip.io", dnsmessage.AResource{A: [4]byte{172, 16, 0, 30}}),
			// dashes
			Entry("shared address with dashes", "100-64-1-2", dnsmessage.AResource{A: [4]byte{100, 64, 1, 2}}),
			Entry("link-local with domain", "169-254-168-253-com", dnsmessage.AResource{A: [4]byte{169, 254, 168, 253}}),
			Entry("IETF protocol assignments with domain and www", "www-192-0-0-1-com", dnsmessage.AResource{A: [4]byte{192, 0, 0, 1}}),
			// dots-and-dashes, mix-and-matches
			Entry("Pandaxin's paradox", "minio-01.192-168-1-100.sslip.io", dnsmessage.AResource{A: [4]byte{192, 168, 1, 100}}),
		)
		DescribeTable("when it does NOT match an IP address",
			func(fqdn string) {
				ipv4Answers := xip.NameToA(fqdn)
				Expect(len(ipv4Answers)).To(Equal(0))
			},
			Entry("empty string", ""),
			Entry("bare domain", "nono.io"),
			Entry("canonical domain", "sslip.io"),
			Entry("www", "www.sslip.io"),
			Entry("a lone number", "538.sslip.io"),
			Entry("too big", "256.254.253.252"),
			Entry("NS but no dot", "ns-aws.sslip.io"),
			Entry("NS + cruft at beginning", "p-ns-aws.sslip.io"),
			Entry("test-net address with dots-and-dashes mixed", "www-192.0-2.3.example-me.com"),
		)
		When("There is more than one A record", func() {
			It("returns them all", func() {
				fqdn := random8ByteString()
				xip.Customizations[strings.ToLower(fqdn)] = xip.DomainCustomization{
					A: []dnsmessage.AResource{
						{A: [4]byte{1}},
						{A: [4]byte{2}},
					},
				}
				ipv4Answers := xip.NameToA(fqdn)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(ipv4Answers)).To(Equal(2))
				Expect(ipv4Answers[0].A).To(Equal([4]byte{1}))
				Expect(ipv4Answers[1].A).To(Equal([4]byte{2}))
				delete(xip.Customizations, fqdn)
			})
		})
		When("There are multiple matches", func() {
			It("returns the leftmost one", func() {
				ipv4Answers := xip.NameToA("nono.io.127.0.0.1.192.168.0.1.sslip.io")
				Expect(len(ipv4Answers)).To(Equal(1))
				Expect(ipv4Answers[0]).
					To(Equal(dnsmessage.AResource{A: [4]byte{127, 0, 0, 1}}))
			})
		})
		When("There are matches with dashes and dots", func() {
			It("returns the one with dashes", func() {
				ipv4Answers := xip.NameToA("nono.io.127.0.0.1.192-168-0-1.sslip.io")
				Expect(len(ipv4Answers)).To(Equal(1))
				Expect(ipv4Answers[0]).
					To(Equal(dnsmessage.AResource{A: [4]byte{192, 168, 0, 1}}))
			})
		})
	})

	Describe("IsAcmeChallenge()", func() {
		When("the domain doesn't have '_acme-challenge.' in it", func() {
			It("returns false", func() {
				randomDomain := random8ByteString() + ".com."
				Expect(xip.IsAcmeChallenge(randomDomain)).To(BeFalse())
			})
			It("returns false even when there are embedded IPs", func() {
				randomDomain := "127.0.0.1." + random8ByteString() + ".com."
				Expect(xip.IsAcmeChallenge(randomDomain)).To(BeFalse())
			})
		})
		When("it has '_acme-challenge.' in it", func() {
			When("it does NOT have any embedded IPs", func() {
				It("returns false", func() {
					randomDomain := "_acme-challenge." + random8ByteString() + ".com."
					Expect(xip.IsAcmeChallenge(randomDomain)).To(BeFalse())
				})
			})
			When("it has embedded IPs", func() {
				It("returns true", func() {
					randomDomain := "_acme-challenge.127.0.0.1." + random8ByteString() + ".com."
					Expect(xip.IsAcmeChallenge(randomDomain)).To(BeTrue())
					randomDomain = "_acme-challenge.fe80--1." + random8ByteString() + ".com."
					Expect(xip.IsAcmeChallenge(randomDomain)).To(BeTrue())
				})
				When("it has random capitalization", func() {
					It("returns true", func() {
						randomDomain := "_AcMe-ChAlLeNgE.127.0.0.1." + random8ByteString() + ".com."
						Expect(xip.IsAcmeChallenge(randomDomain)).To(BeTrue())
						randomDomain = "_aCMe-cHAllENge.fe80--1." + random8ByteString() + ".com."
						Expect(xip.IsAcmeChallenge(randomDomain)).To(BeTrue())
					})
				})
			})
		})
	})

	Describe("NameToAAAA()", func() {
		DescribeTable("when it succeeds",
			func(fqdn string, expectedAAAA dnsmessage.AAAAResource) {
				ipv6Answers := xip.NameToAAAA(fqdn)
				Expect(len(ipv6Answers)).To(Equal(1))
				Expect(ipv6Answers[0]).To(Equal(expectedAAAA))
			},
			// dashes only
			Entry("loopback", "--1", dnsmessage.AAAAResource{AAAA: [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}}),
			Entry("ff with domain", "fffe-fdfc-fbfa-f9f8-f7f6-f5f4-f3f2-f1f0.com", dnsmessage.AAAAResource{AAAA: [16]byte{255, 254, 253, 252, 251, 250, 249, 248, 247, 246, 245, 244, 243, 242, 241, 240}}),
			Entry("ff with domain and pre", "www.fffe-fdfc-fbfa-f9f8-f7f6-f5f4-f3f2-f1f0.com", dnsmessage.AAAAResource{AAAA: [16]byte{255, 254, 253, 252, 251, 250, 249, 248, 247, 246, 245, 244, 243, 242, 241, 240}}),
			Entry("ff with domain dashes", "1.www-fffe-fdfc-fbfa-f9f8-f7f6-f5f4-f3f2-f1f0-1.com", dnsmessage.AAAAResource{AAAA: [16]byte{255, 254, 253, 252, 251, 250, 249, 248, 247, 246, 245, 244, 243, 242, 241, 240}}),
			Entry("Browsing the logs", "2006-41d0-2-e01e--56dB-3598.sSLIP.io.", dnsmessage.AAAAResource{AAAA: [16]byte{32, 6, 65, 208, 0, 2, 224, 30, 0, 0, 0, 0, 86, 219, 53, 152}}),
			Entry("Browsing the logs", "1-2-3--4-5-6.sSLIP.io.", dnsmessage.AAAAResource{AAAA: [16]byte{0, 1, 0, 2, 0, 3, 0, 0, 0, 0, 0, 4, 0, 5, 0, 6}}),
			Entry("Browsing the logs", "1--2-3-4-5-6.sSLIP.io.", dnsmessage.AAAAResource{AAAA: [16]byte{0, 1, 0, 0, 0, 0, 0, 2, 0, 3, 0, 4, 0, 5, 0, 6}}),
		)
		DescribeTable("when it does not match an IP address",
			func(fqdn string) {
				ipv6Answers := xip.NameToAAAA(fqdn)
				Expect(len(ipv6Answers)).To(Equal(0))
			},
			Entry("empty string", ""),
			Entry("bare domain", "nono.io"),
			Entry("canonical domain", "sslip.io"),
			Entry("www", "www.sslip.io"),
			Entry("a 1 without double-dash", "-1"),
			Entry("too big", "--g"),
		)
		When("using randomly generated IPv6 addresses (fuzz testing)", func() {
			It("should succeed every time", func() {
				for i := 0; i < 10000; i++ {
					addr := randomIPv6Address()
					ipv6Answers := xip.NameToAAAA(strings.ReplaceAll(addr.String(), ":", "-"))
					Expect(err).ToNot(HaveOccurred())
					Expect(ipv6Answers[0].AAAA[:]).To(Equal([]uint8(addr)))
				}
			})
		})
		When("There is more than one AAAA record", func() {
			It("returns them all", func() {
				fqdn := random8ByteString()
				xip.Customizations[strings.ToLower(fqdn)] = xip.DomainCustomization{
					AAAA: []dnsmessage.AAAAResource{
						{AAAA: [16]byte{1}},
						{AAAA: [16]byte{2}},
					},
				}
				ipv6Addrs := xip.NameToAAAA(fqdn)
				Expect(len(ipv6Addrs)).To(Equal(2))
				Expect(ipv6Addrs[0].AAAA).To(Equal([16]byte{1}))
				Expect(ipv6Addrs[1].AAAA).To(Equal([16]byte{2}))
				delete(xip.Customizations, fqdn)
			})
		})
	})

	Describe("ReadBlocklist()", func() {
		It("strips comments", func() {
			input := strings.NewReader("# a comment\n#another comment\nno-comments\n")
			bls, blIPs, err := xip.ReadBlocklist(input)
			Expect(err).ToNot(HaveOccurred())
			Expect(bls).To(Equal([]string{"no-comments"}))
			Expect(blIPs).To(BeNil())
		})
		It("strips blank lines", func() {
			input := strings.NewReader("\n\n\nno-blank-lines")
			bls, blIPs, err := xip.ReadBlocklist(input)
			Expect(err).ToNot(HaveOccurred())
			Expect(bls).To(Equal([]string{"no-blank-lines"}))
			Expect(blIPs).To(BeNil())
		})
		It("lowercases names for comparison", func() {
			input := strings.NewReader("NO-YELLING")
			bls, blIPs, err := xip.ReadBlocklist(input)
			Expect(err).ToNot(HaveOccurred())
			Expect(bls).To(Equal([]string{"no-yelling"}))
			Expect(blIPs).To(BeNil())
		})
		It("removes all non-allowable characters", func() {
			input := strings.NewReader("\nalpha #comment # comment\nåß∂ # comment # comment\ndelta∆\n ... GAMMA∑µ®† ...#asdfasdf#asdfasdf")
			bls, blIPs, err := xip.ReadBlocklist(input)
			Expect(err).ToNot(HaveOccurred())
			Expect(bls).To(Equal([]string{"alpha", "delta", "gamma"}))
			Expect(blIPs).To(BeNil())
		})
		It("reads in IPv4 CIDRs", func() {
			input := strings.NewReader("\n43.134.66.67/24 #asdfasdf")
			bls, blIPs, err := xip.ReadBlocklist(input)
			Expect(err).ToNot(HaveOccurred())
			Expect(bls).To(BeNil())
			Expect(blIPs).To(Equal([]net.IPNet{{IP: net.IP{43, 134, 66, 0}, Mask: net.IPMask{255, 255, 255, 0}}}))
		})
		It("reads in IPv6 CIDRs", func() {
			input := strings.NewReader("\n 2600::/64 #asdfasdf")
			bls, blIPs, err := xip.ReadBlocklist(input)
			Expect(err).ToNot(HaveOccurred())
			Expect(bls).To(BeNil())
			Expect(blIPs).To(Equal([]net.IPNet{
				{IP: net.IP{38, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
					Mask: net.IPMask{255, 255, 255, 255, 255, 255, 255, 255, 0, 0, 0, 0, 0, 0, 0, 0}}}))
		})
	})
})

func randomIPv6Address() net.IP {
	upperHalf := make([]byte, 8)
	lowerHalf := make([]byte, 8)
	binary.LittleEndian.PutUint64(upperHalf, rand.Uint64())
	binary.LittleEndian.PutUint64(lowerHalf, rand.Uint64())
	ipv6 := net.IP(append(upperHalf, lowerHalf...))
	// IPv6 addrs have a lot of all-zero two-byte sections
	// So we zero-out ~50% of the sections
	for i := 0; i < 8; i++ {
		if rand.Int()%2 == 0 {
			for j := 0; j < 2; j++ {
				ipv6[i*2+j] = 0
			}
		}
	}
	// avoid pathological case: an IPv4 address []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff, ?, ?, ?, ?})
	ipv6[10] &= 0xfe
	return ipv6
}

// random8ByteString() returns an 8-char mixed-case string consisting solely of the letters a-z.
func random8ByteString() string {
	var randomString []byte
	for i := 0; i < 8; i++ {
		// 65 == ascii 'A', +32 (96) == ascii 'a', there are 26 letters in the alphabet. Mix upper case, too.
		randomString = append(randomString, byte(65+32*rand.Intn(2)+rand.Intn(26)))
	}
	return string(randomString)
}
